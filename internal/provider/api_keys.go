package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

type apiKeyModel struct {
	Name      types.String `tfsdk:"name"`
	Key       types.String `tfsdk:"key"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func mapToAPIKeyModels(apiKeys []api.APIKey) []apiKeyModel {
	keyModels := make([]apiKeyModel, 0, len(apiKeys))
	for _, key := range apiKeys {
		keyModel := apiKeyModel{
			Name:      types.StringValue(key.Name),
			Key:       types.StringValue(key.Key),
			CreatedAt: types.StringValue(key.CreatedAt),
		}

		keyModels = append(keyModels, keyModel)
	}

	return keyModels
}
