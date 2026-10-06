package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newCaptureKeyServer(t *testing.T, captured *APIKeyCreateRequest) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimPrefix(r.URL.Path, "/api/v1/") != "create_api_key_v2" {
			t.Errorf("unexpected path %s", r.URL.Path)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(captured)
		writeJSON(w, APIKey{ID: "aki_1"})
	}))
	t.Cleanup(server.Close)

	token := "aks_test"
	c, err := NewClient(server.URL+"/api/v1", &token, "test")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCreateApplicationKeyV2ReadOnlySendsReadOnlyGrant(t *testing.T) {
	var keyReq APIKeyCreateRequest
	c := newCaptureKeyServer(t, &keyReq)

	if _, err := c.CreateApplicationKeyV2("akn_test", "wi_1", true); err != nil {
		t.Fatal(err)
	}
	if len(keyReq.AccessGrants) != 1 || keyReq.AccessGrants[0]["principal_kind"] != PrincipalKindApplicationReadOnly ||
		keyReq.AccessGrants[0]["workspace_id"] != "wi_1" {
		t.Fatalf("unexpected create_api_key_v2 grants %v", keyReq.AccessGrants)
	}
}

func TestCreateClusterScopedApplicationKeyV2SendsClusterGrant(t *testing.T) {
	var keyReq APIKeyCreateRequest
	c := newCaptureKeyServer(t, &keyReq)

	if _, err := c.CreateClusterScopedApplicationKeyV2("akn_test", "wi_1", testVCID, ResourceKindVirtualClusterTopics); err != nil {
		t.Fatal(err)
	}
	grant := keyReq.AccessGrants[0]
	if len(keyReq.AccessGrants) != 1 || grant["principal_kind"] != PrincipalKindApplication ||
		grant["resource_kind"] != ResourceKindVirtualClusterTopics || grant["resource_id"] != testVCID {
		t.Fatalf("unexpected create_api_key_v2 grants %v", keyReq.AccessGrants)
	}

	if _, err := c.CreateClusterScopedApplicationKeyV2("akn_test", "wi_1", testVCID, ResourceKindVirtualCluster); err == nil {
		t.Fatal("expected an error for a resource kind that isn't a cluster sub-resource")
	}
}
