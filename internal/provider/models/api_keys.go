package models

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

type ApplicationKey struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Key              types.String `tfsdk:"key"`
	WorkspaceID      types.String `tfsdk:"workspace_id"`
	VirtualClusterID types.String `tfsdk:"virtual_cluster_id"`
	ResourceKind     types.String `tfsdk:"resource_kind"`
	CreatedAt        types.String `tfsdk:"created_at"`
	ReadOnly         types.Bool   `tfsdk:"read_only"`
}

// Ideally AgentKey and ApplicationKey would share fields by composing an APIKey struct.
// But I'm not sure how to make struct composition work with setting state on the TF response object.
type AgentKey struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Key       types.String `tfsdk:"key"`
	CreatedAt types.String `tfsdk:"created_at"`

	VirtualClusterID types.String `tfsdk:"virtual_cluster_id"`
	ReadOnly         types.Bool   `tfsdk:"read_only"`
}

func MapToApplicationKeys(apiKeysPtr *[]api.APIKey) *[]ApplicationKey {
	apiKeys := *apiKeysPtr

	keyModels := make([]ApplicationKey, 0, len(apiKeys))
	for _, key := range apiKeys {
		keyModel := ApplicationKeyFromAPI(key)
		keyModels = append(keyModels, keyModel)
	}

	return &keyModels
}

// ApplicationKeyFromAPI maps an API key response to the Terraform application key model.
func ApplicationKeyFromAPI(key api.APIKey) ApplicationKey {
	model := ApplicationKey{
		ID:          types.StringValue(key.ID),
		Name:        types.StringValue(key.Name),
		Key:         types.StringValue(key.Key),
		WorkspaceID: types.StringValue(key.AccessGrants.ReadWorkspaceIDSafe()),
		CreatedAt:   types.StringValue(key.CreatedAt),
		ReadOnly:    types.BoolValue(key.IsReadOnly()),
	}

	if vcID, resourceKind, ok := key.ApplicationKeyClusterScope(); ok {
		model.VirtualClusterID = types.StringValue(vcID)
		model.ResourceKind = types.StringValue(resourceKind)
	} else {
		model.VirtualClusterID = types.StringNull()
		model.ResourceKind = types.StringNull()
	}

	return model
}

func MapToAgentKeys(apiKeysPtr *[]api.APIKey, diags *diag.Diagnostics) (*[]AgentKey, bool) {
	if apiKeysPtr == nil {
		// Null for Serverless clusters.
		return nil, true
	}

	apiKeys := *apiKeysPtr

	keyModels := make([]AgentKey, 0, len(apiKeys))
	for _, key := range apiKeys {
		vcID, ok := key.GetVirtualClusterID(diags)
		if !ok {
			return nil, false // Diagnostics handled by helper.
		}
		keyModel := AgentKey{
			ID:               types.StringValue(key.ID),
			Name:             types.StringValue(key.Name),
			Key:              types.StringValue(key.Key),
			VirtualClusterID: types.StringValue(vcID),
			CreatedAt:        types.StringValue(key.CreatedAt),
			ReadOnly:         types.BoolValue(key.IsReadOnly()),
		}

		keyModels = append(keyModels, keyModel)
	}

	return &keyModels, true
}
