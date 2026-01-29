package models

import "github.com/hashicorp/terraform-plugin-framework/types"

type SSOConfiguration struct {
	ID                   types.String `tfsdk:"id"`
	SSOIdentifier        types.String `tfsdk:"sso_identifier"`
	EntityID             types.String `tfsdk:"entity_id"`
	SAMLURL              types.String `tfsdk:"saml_url"`
	DefaultRoleID        types.String `tfsdk:"default_role_id"`
	EnableSSORoleMapping types.Bool   `tfsdk:"enable_sso_role_mapping"`
	SigningCertificate   types.String `tfsdk:"signing_certificate"`
}
