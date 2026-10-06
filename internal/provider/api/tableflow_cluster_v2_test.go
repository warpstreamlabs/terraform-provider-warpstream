package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTableFlowV2CreatesSendTheTableFlowType(t *testing.T) {
	var clusterReq VirtualClusterCreateRequest
	var keyReq APIKeyCreateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/api/v1/") {
		case "create_virtual_cluster_v2":
			_ = json.NewDecoder(r.Body).Decode(&clusterReq)
			writeJSON(w, VirtualClusterCreateResponse{VirtualClusterID: "vci_dl_1"})
		case "create_api_key_v2":
			_ = json.NewDecoder(r.Body).Decode(&keyReq)
			writeJSON(w, APIKey{ID: "aki_1"})
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

	if _, _, err := c.CreateVirtualClusterV2("vcn_dl_test", ClusterParameters{Type: VirtualClusterTypeTableFlow}); err != nil {
		t.Fatal(err)
	}
	if clusterReq.Name != "test" || clusterReq.Type != VirtualClusterTypeTableFlow {
		t.Fatalf("unexpected create_virtual_cluster_v2 request %+v", clusterReq)
	}

	if _, err := c.CreateAgentKeyV2("akn_test", "vci_dl_1"); err != nil {
		t.Fatal(err)
	}
	if keyReq.VirtualClusterTypeOverride != VirtualClusterTypeTableFlow {
		t.Fatalf("unexpected create_api_key_v2 type %q", keyReq.VirtualClusterTypeOverride)
	}
}
