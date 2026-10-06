package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSchemaRegistryV2CreatesSendTheSchemaRegistryType(t *testing.T) {
	var clusterReq VirtualClusterCreateRequest
	var keyReq APIKeyCreateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/api/v1/") {
		case "create_virtual_cluster_v2":
			_ = json.NewDecoder(r.Body).Decode(&clusterReq)
			writeJSON(w, VirtualClusterCreateResponse{VirtualClusterID: "vci_sr_1"})
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

	if _, _, err := c.CreateVirtualClusterV2("vcn_sr_test", ClusterParameters{Type: VirtualClusterTypeSchemaRegistry}); err != nil {
		t.Fatal(err)
	}
	if clusterReq.Name != "test" || clusterReq.Type != VirtualClusterTypeSchemaRegistry {
		t.Fatalf("unexpected create_virtual_cluster_v2 request %+v", clusterReq)
	}

	if _, err := c.CreateAgentKeyV2("akn_test", "vci_sr_1", false); err != nil {
		t.Fatal(err)
	}
	if keyReq.VirtualClusterTypeOverride != VirtualClusterTypeSchemaRegistry {
		t.Fatalf("unexpected create_api_key_v2 type %q", keyReq.VirtualClusterTypeOverride)
	}
}
