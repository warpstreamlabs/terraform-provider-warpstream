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

const testVCID = "vci_11111111_1111_1111_1111_111111111111"

// fakeAPIKeyServer scripts create_api_key_v2 responses and serves list_api_keys and delete_api_key.
type fakeAPIKeyServer struct {
	mu        sync.Mutex
	responses []func(w http.ResponseWriter, s *fakeAPIKeyServer)
	listed    []APIKey
	hits      map[string]int
	deleted   []string
}

func newFakeAPIKeyServer(t *testing.T, responses ...func(w http.ResponseWriter, s *fakeAPIKeyServer)) (*fakeAPIKeyServer, *Client) {
	t.Helper()
	s := &fakeAPIKeyServer{responses: responses, hits: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		s.hits[path]++
		switch path {
		case "create_api_key_v2":
			if len(s.responses) == 0 {
				t.Errorf("unexpected %s call", path)
				w.WriteHeader(http.StatusTeapot)
				return
			}
			respond := s.responses[0]
			s.responses = s.responses[1:]
			respond(w, s)
		case "list_api_keys":
			writeJSON(w, APIKeyListResponse{APIKeys: s.listed})
		case "delete_api_key":
			var req APIKeyDeleteRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			s.deleted = append(s.deleted, req.ID)
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
	return s, c
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func agentKey(id string, createdAt time.Time) APIKey {
	return APIKey{
		ID:        id,
		Name:      "akn_test",
		Key:       "aks_v2_secret_" + id,
		CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
		AccessGrants: AccessGrants{{
			PrincipalKind: PrincipalKindAgent,
			ResourceKind:  ResourceKindVirtualCluster,
			ResourceID:    testVCID,
		}},
	}
}

func created(id string) func(w http.ResponseWriter, s *fakeAPIKeyServer) {
	return func(w http.ResponseWriter, s *fakeAPIKeyServer) {
		key := agentKey(id, time.Now())
		s.listed = append(s.listed, key)
		writeJSON(w, key)
	}
}

// createdThen500 commits the key but fails the response, like a proxy timing out after forwarding.
func createdThen500(id string) func(w http.ResponseWriter, s *fakeAPIKeyServer) {
	return func(w http.ResponseWriter, s *fakeAPIKeyServer) {
		s.listed = append(s.listed, agentKey(id, time.Now()))
		w.WriteHeader(http.StatusBadGateway)
	}
}

func status(code int) func(w http.ResponseWriter, s *fakeAPIKeyServer) {
	return func(w http.ResponseWriter, _ *fakeAPIKeyServer) {
		w.WriteHeader(code)
	}
}

func dropConnection(w http.ResponseWriter, _ *fakeAPIKeyServer) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hijacker.Hijack()
	if err == nil {
		_ = conn.Close()
	}
}

func TestCreateAPIKeyV2ReturnsCreateResponseSecret(t *testing.T) {
	s, c := newFakeAPIKeyServer(t, created("aki_1"))

	key, err := c.CreateAgentKeyV2("akn_test", testVCID, false)
	if err != nil {
		t.Fatal(err)
	}
	if key.Key != "aks_v2_secret_aki_1" {
		t.Fatalf("unexpected secret %q", key.Key)
	}
	if s.hits["create_api_key_v2"] != 1 || s.hits["list_api_keys"] != 0 {
		t.Fatalf("unexpected hits %v", s.hits)
	}
}

func TestCreateAPIKeyV2DoesNotRetryRejection(t *testing.T) {
	s, c := newFakeAPIKeyServer(t, status(http.StatusBadRequest))

	_, err := c.CreateAgentKeyV2("akn_test", testVCID, false)
	if err == nil || errors.Is(err, ErrAmbiguous) {
		t.Fatalf("expected a plain error, got %v", err)
	}
	if s.hits["create_api_key_v2"] != 1 || s.hits["list_api_keys"] != 0 {
		t.Fatalf("unexpected hits %v", s.hits)
	}
}

func TestCreateAPIKeyV2DeletesOrphanAndRetries(t *testing.T) {
	s, c := newFakeAPIKeyServer(t, createdThen500("aki_orphan"), created("aki_2"))

	key, err := c.CreateAgentKeyV2("akn_test", testVCID, false)
	if err != nil {
		t.Fatal(err)
	}
	if key.ID != "aki_2" || key.Key != "aks_v2_secret_aki_2" {
		t.Fatalf("unexpected key %+v", key)
	}
	if len(s.deleted) != 1 || s.deleted[0] != "aki_orphan" {
		t.Fatalf("expected the orphan to be deleted, got %v", s.deleted)
	}
	if s.hits["create_api_key_v2"] != 2 {
		t.Fatalf("unexpected hits %v", s.hits)
	}
}

func TestCreateAPIKeyV2RetriesWhenNothingWasCreated(t *testing.T) {
	s, c := newFakeAPIKeyServer(t, dropConnection, created("aki_1"))

	if _, err := c.CreateAgentKeyV2("akn_test", testVCID, false); err != nil {
		t.Fatal(err)
	}
	if len(s.deleted) != 0 || s.hits["create_api_key_v2"] != 2 || s.hits["list_api_keys"] != 1 {
		t.Fatalf("unexpected hits %v, deleted %v", s.hits, s.deleted)
	}
}

func TestCreateAPIKeyV2KeepsKeysItDidNotCreate(t *testing.T) {
	otherGrant := agentKey("aki_other_grant", time.Now())
	otherGrant.AccessGrants[0].ResourceID = "vci_22222222_2222_2222_2222_222222222222"

	for name, existing := range map[string]APIKey{
		"created before the request": agentKey("aki_old", time.Now().Add(-time.Hour)),
		"different access grant":     otherGrant,
	} {
		t.Run(name, func(t *testing.T) {
			s, c := newFakeAPIKeyServer(t, status(http.StatusInternalServerError))
			s.listed = []APIKey{existing}

			_, err := c.CreateAgentKeyV2("akn_test", testVCID, false)
			if !errors.Is(err, ErrAmbiguous) || !strings.Contains(err.Error(), "delete any key named akn_test") {
				t.Fatalf("expected a manual cleanup error, got %v", err)
			}
			if len(s.deleted) != 0 || s.hits["create_api_key_v2"] != 1 {
				t.Fatalf("unexpected hits %v, deleted %v", s.hits, s.deleted)
			}
		})
	}
}

func TestCreateAPIKeyV2GivesUpAfterSecondAmbiguousFailure(t *testing.T) {
	s, c := newFakeAPIKeyServer(t, status(http.StatusInternalServerError), status(499))

	_, err := c.CreateAgentKeyV2("akn_test", testVCID, false)
	if !errors.Is(err, ErrAmbiguous) || !strings.Contains(err.Error(), "delete any key named akn_test") {
		t.Fatalf("expected a manual cleanup error, got %v", err)
	}
	if s.hits["create_api_key_v2"] != 2 {
		t.Fatalf("unexpected hits %v", s.hits)
	}
}
