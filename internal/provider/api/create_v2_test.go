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

// fakeV2CreateServer fails the first create_virtual_cluster_v2 call with a 502 after
// committing the object, like a proxy timing out after forwarding, then succeeds.
type fakeV2CreateServer struct {
	mu       sync.Mutex
	hits     map[string]int
	clusters []VirtualCluster
	deleted  []string
}

func newFakeV2CreateServer(t *testing.T) (*fakeV2CreateServer, *Client) {
	t.Helper()
	s := &fakeV2CreateServer{hits: map[string]int{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
		s.hits[path]++
		now := time.Now().UTC().Format(time.RFC3339Nano)
		switch path {
		case "create_virtual_cluster_v2":
			id := "vci_" + strings.Repeat("1", s.hits[path])
			s.clusters = append(s.clusters, VirtualCluster{ID: id, Name: "vcn_test", Type: VirtualClusterTypeBYOC, CreatedAt: now})
			if s.hits[path] == 1 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			writeJSON(w, VirtualClusterCreateResponse{VirtualClusterID: id, Name: "vcn_test", AgentKey: APIKey{ID: "aki_1", Key: "aks_v2_secret"}})
		case "list_virtual_clusters":
			writeJSON(w, VirtualClusterListResponse{VirtualClusters: s.clusters})
		case "delete_virtual_cluster":
			var req VirtualClusterDeleteRequest
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

func TestCreateVirtualClusterV2DeletesOrphanAndRetries(t *testing.T) {
	s, c := newFakeV2CreateServer(t)

	vc, key, err := c.CreateVirtualClusterV2("vcn_test", ClusterParameters{Type: VirtualClusterTypeBYOC})
	if err != nil {
		t.Fatal(err)
	}
	if vc.ID != "vci_11" || key.Key != "aks_v2_secret" {
		t.Fatalf("unexpected cluster %+v, key %+v", vc, key)
	}
	if len(s.deleted) != 1 || s.deleted[0] != "vci_1" || s.hits["create_virtual_cluster_v2"] != 2 {
		t.Fatalf("unexpected hits %v, deleted %v", s.hits, s.deleted)
	}
}

func TestCreateVirtualClusterV2KeepsPreexistingCluster(t *testing.T) {
	s, c := newFakeV2CreateServer(t)
	s.clusters = []VirtualCluster{{ID: "vci_old", Name: "vcn_test", Type: VirtualClusterTypeBYOC,
		CreatedAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)}}

	_, _, err := c.CreateVirtualClusterV2("vcn_test", ClusterParameters{Type: VirtualClusterTypeBYOC})
	if !errors.Is(err, ErrAmbiguous) || !strings.Contains(err.Error(), "delete any virtual cluster named vcn_test") {
		t.Fatalf("expected a manual cleanup error, got %v", err)
	}
	if len(s.deleted) != 0 || s.hits["create_virtual_cluster_v2"] != 1 {
		t.Fatalf("unexpected hits %v, deleted %v", s.hits, s.deleted)
	}
}
