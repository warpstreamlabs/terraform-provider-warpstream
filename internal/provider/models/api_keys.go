package models

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

type ApplicationKey struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Key         types.String `tfsdk:"key"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

// Ideally AgentKey and ApplicationKey would share fields by composing an APIKey struct.
// But I'm not sure how to make struct composition work with setting state on the TF response object.
type AgentKey struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Key       types.String `tfsdk:"key"`
	CreatedAt types.String `tfsdk:"created_at"`

	VirtualClusterID types.String `tfsdk:"virtual_cluster_id"`
}

func MapToApplicationKeys(apiKeysPtr *[]api.APIKey) *[]ApplicationKey {
	apiKeys := *apiKeysPtr

	keyModels := make([]ApplicationKey, 0, len(apiKeys))
	for _, key := range apiKeys {
		keyModel := ApplicationKey{
			ID:          types.StringValue(key.ID),
			Name:        types.StringValue(key.Name),
			Key:         types.StringValue(key.Key),
			WorkspaceID: types.StringValue((key.AccessGrants.ReadWorkspaceIDSafe())),
			CreatedAt:   types.StringValue(key.CreatedAt),
		}

		keyModels = append(keyModels, keyModel)
	}

	return &keyModels
}

func MapToAgentKeys(apiKeysPtr *[]api.APIKey, diags *diag.Diagnostics) (*[]AgentKey, bool) {
	if apiKeysPtr == nil {
		// Null for Serverless clusters.
		return nil, true
	}

	apiKeys := *apiKeysPtr

	keyModels := make([]AgentKey, 0, len(apiKeys))
	for _, key := range apiKeys {
		vcID, ok := GetVirtualClusterID(key, diags)
		if !ok {
			return nil, false // Diagnostics handled by helper.
		}
		keyModel := AgentKey{
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

// TODO simon: make this a method on the api.APIKey struct? maybe in a later PR.
func GetVirtualClusterID(apiKey api.APIKey, diags *diag.Diagnostics) (string, bool) {
	if len(apiKey.AccessGrants) == 0 {
		diags.AddError(
			"Error Reading WarpStream Agent Key",
			"API returned invalid Agent Key with ID "+apiKey.ID+": no access grants found",
		)
		return "", false
	}

	return apiKey.AccessGrants[0].ResourceID, true
}
