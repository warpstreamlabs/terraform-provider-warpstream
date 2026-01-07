package tests

import (
	"testing"

	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

func cleanupAPIKeyByName(t *testing.T, name string) {
	t.Helper()

	client, err := api.NewClientDefault()
	if err != nil {
		t.Errorf("cleanup: failed to create api client: %v", err)
		return
	}

	apiKeys, err := client.GetAPIKeys()
	if err != nil {
		t.Errorf("cleanup: failed to list api keys: %v", err)
		return
	}

	for _, apiKey := range apiKeys {
		if apiKey.Name != name {
			continue
		}
		if err := client.DeleteAPIKey(apiKey.ID); err != nil {
			t.Errorf("cleanup: failed to delete api key %q (%s): %v", name, apiKey.ID, err)
		}
		return
	}
}
