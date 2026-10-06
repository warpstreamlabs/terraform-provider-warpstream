package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCreateWorkspaceV2DeletesOrphanAndRetries fails the first create_workspace_v2 call with a 502 after
// committing the workspace, like a proxy timing out after forwarding, then succeeds.
func TestCreateWorkspaceV2DeletesOrphanAndRetries(t *testing.T) {
	var (
		mu         sync.Mutex
		hits       = map[string]int{}
		workspaces []Workspace
		deleted    []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		hits[path]++
		switch path {
		case "create_workspace_v2":
			id := "wi_" + strings.Repeat("1", hits[path])
			workspaces = append(workspaces, Workspace{ID: id, Name: "ws", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
			if hits[path] == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			writeJSON(w, workspaceCreateV2Response{ID: id, ApplicationKey: &APIKey{ID: "aki_1", Key: "aks_v2_secret"}})
		case "list_workspaces":
			writeJSON(w, WorkspaceListResponse{Workspaces: workspaces})
		case "delete_workspace":
			var req WorkspaceDeleteRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			deleted = append(deleted, req.ID)
			_, _ = w.Write([]byte("{}"))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	token := "aks_test"
	c, err := NewClient(server.URL+"/api/v1", &token, "test")
	if err != nil {
		t.Fatal(err)
	}

	id, key, err := c.CreateWorkspaceV2("ws")
	if err != nil {
		t.Fatal(err)
	}
	if id != "wi_11" || key.Key != "aks_v2_secret" {
		t.Fatalf("unexpected workspace %s, key %+v", id, key)
	}
	if len(deleted) != 1 || deleted[0] != "wi_1" || hits["create_workspace_v2"] != 2 {
		t.Fatalf("unexpected hits %v, deleted %v", hits, deleted)
	}
}

func TestCreateApplicationKeyV2KeepsKeyOfOtherWorkspace(t *testing.T) {
	s, c := newFakeAPIKeyServer(t, status(http.StatusInternalServerError))
	s.listed = []APIKey{{
		ID:        "aki_other_workspace",
		Name:      "akn_test",
		CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		AccessGrants: AccessGrants{{
			PrincipalKind: PrincipalKindApplication,
			ResourceKind:  ResourceKindAny,
			ResourceID:    ResourceIDAny,
			WorkspaceID:   "wi_other",
		}},
	}}

	_, err := c.CreateApplicationKeyV2("akn_test", "wi_1")
	if !errors.Is(err, ErrAmbiguous) || !strings.Contains(err.Error(), "delete any key named akn_test") {
		t.Fatalf("expected a manual cleanup error, got %v", err)
	}
	if len(s.deleted) != 0 || s.hits["create_api_key_v2"] != 1 {
		t.Fatalf("unexpected hits %v, deleted %v", s.hits, s.deleted)
	}
}
