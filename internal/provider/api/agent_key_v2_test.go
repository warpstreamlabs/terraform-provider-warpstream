package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateAgentKeyV2ReadOnlySendsReadOnlyGrant(t *testing.T) {
	var keyReq APIKeyCreateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimPrefix(r.URL.Path, "/api/v1/") != "create_api_key_v2" {
			t.Errorf("unexpected path %s", r.URL.Path)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&keyReq)
		writeJSON(w, APIKey{ID: "aki_1"})
	}))
	t.Cleanup(server.Close)

	token := "aks_test"
	c, err := NewClient(server.URL+"/api/v1", &token, "test")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.CreateAgentKeyV2("akn_test", testVCID, true); err != nil {
		t.Fatal(err)
	}
	if len(keyReq.AccessGrants) != 1 || keyReq.AccessGrants[0]["principal_kind"] != PrincipalKindAgentReadOnly {
		t.Fatalf("unexpected create_api_key_v2 grants %v", keyReq.AccessGrants)
	}
}
