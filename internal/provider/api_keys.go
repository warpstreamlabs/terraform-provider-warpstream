package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

type applicationKeyModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Key       types.String `tfsdk:"key"`
	CreatedAt types.String `tfsdk:"created_at"`
}

// Ideally agentKeyModel and applicationKeyModel would share fields by composing an apiKeyModel struct.
// But I'm not sure how to make struct composition work with setting state on the TF response object.
type agentKeyModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Key       types.String `tfsdk:"key"`
	CreatedAt types.String `tfsdk:"created_at"`

	VirtualClusterID types.String `tfsdk:"virtual_cluster_id"`
}

func mapToApplicationKeyModels(apiKeysPtr *[]api.APIKey) *[]applicationKeyModel {
	apiKeys := *apiKeysPtr

	keyModels := make([]applicationKeyModel, 0, len(apiKeys))
	for _, key := range apiKeys {
		keyModel := applicationKeyModel{
			ID:        types.StringValue(key.ID),
			Name:      types.StringValue(key.Name),
			Key:       types.StringValue(key.Key),
			CreatedAt: types.StringValue(key.CreatedAt),
		}

		keyModels = append(keyModels, keyModel)
	}

	return &keyModels
}

func mapToAgentKeyModels(apiKeysPtr *[]api.APIKey, diags *diag.Diagnostics) (*[]agentKeyModel, bool) {
	if apiKeysPtr == nil {
		// Null for Serverless clusters.
		return nil, true
	}

	apiKeys := *apiKeysPtr

	keyModels := make([]agentKeyModel, 0, len(apiKeys))
	for _, key := range apiKeys {
		vcID, ok := getVirtualClusterID(key, diags)
		if !ok {
			return nil, false // Diagnostics handled by helper.
		}
		keyModel := agentKeyModel{
			ID:               types.StringValue(key.ID),
			Name:             types.StringValue(key.Name),
			Key:              types.StringValue(key.Key),
			VirtualClusterID: types.StringValue(vcID),
			CreatedAt:        types.StringValue(key.CreatedAt),
		}

		keyModels = append(keyModels, keyModel)
	}

	return &keyModels, true
}

func getVirtualClusterID(apiKey api.APIKey, diags *diag.Diagnostics) (string, bool) {
	if len(apiKey.AccessGrants) == 0 {
		diags.AddError(
			"Error Reading WarpStream Agent Key",
			"API returned invalid Agent Key with ID "+apiKey.ID+": no access grants found",
		)
		return "", false
	}

	return apiKey.AccessGrants[0].ResourceID, true
}
