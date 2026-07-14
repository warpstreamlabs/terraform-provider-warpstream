package models

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

type ClaimMatch struct {
	ClaimPath     types.String `tfsdk:"claim_path"`
	ExpectedValue types.String `tfsdk:"expected_value"`
}

type WorkloadIdentityFederation struct {
	ID                      types.String `tfsdk:"id"`
	VirtualClusterID        types.String `tfsdk:"virtual_cluster_id"`
	Name                    types.String `tfsdk:"name"`
	IssuerURL               types.String `tfsdk:"issuer_url"`
	Audience                types.String `tfsdk:"audience"`
	ClaimMatchRules         []ClaimMatch `tfsdk:"claim_match_rules"`
	ReadOnly                types.Bool   `tfsdk:"read_only"`
	MaxCredentialTTLSeconds types.Int64  `tfsdk:"max_credential_ttl_seconds"`
	CreatedAt               types.String `tfsdk:"created_at"`
}

// ToAPIClaimMatchRules converts the model's claim rules into the API representation.
func (m WorkloadIdentityFederation) ToAPIClaimMatchRules() []api.ClaimMatch {
	rules := make([]api.ClaimMatch, 0, len(m.ClaimMatchRules))
	for _, rule := range m.ClaimMatchRules {
		rules = append(rules, api.ClaimMatch{
			ClaimPath:     rule.ClaimPath.ValueString(),
			ExpectedValue: rule.ExpectedValue.ValueString(),
		})
	}
	return rules
}

// MapToWorkloadIdentityFederation maps an API federation binding into the Terraform model.
func MapToWorkloadIdentityFederation(fed *api.WorkloadIdentityFederation) WorkloadIdentityFederation {
	rules := make([]ClaimMatch, 0, len(fed.ClaimMatchRules))
	for _, rule := range fed.ClaimMatchRules {
		rules = append(rules, ClaimMatch{
			ClaimPath:     types.StringValue(rule.ClaimPath),
			ExpectedValue: types.StringValue(rule.ExpectedValue),
		})
	}

	return WorkloadIdentityFederation{
		ID:                      types.StringValue(fed.ID),
		VirtualClusterID:        types.StringValue(fed.VirtualClusterID),
		Name:                    types.StringValue(fed.Name),
		IssuerURL:               types.StringValue(fed.IssuerURL),
		Audience:                types.StringValue(fed.Audience),
		ClaimMatchRules:         rules,
		ReadOnly:                types.BoolValue(fed.ReadOnly),
		MaxCredentialTTLSeconds: types.Int64Value(fed.MaxCredentialTTLSeconds),
		CreatedAt:               types.StringValue(fed.CreatedAt),
	}
}
