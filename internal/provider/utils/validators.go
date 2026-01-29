package utils

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func StartsWithAndAlphanumeric(prefix string) validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(fmt.Sprintf("^%s[a-zA-Z0-9_]+$", prefix)),
		fmt.Sprintf("must start with '%s' and must contain underscores and alphanumeric characters only", prefix),
	)
}

func ValidClusterID() validator.String {
	return stringvalidator.All(
		StartsWithAndAlphanumeric("vci_"),
	)
}

func ValidClusterName() validator.String {
	return stringvalidator.All(
		StartsWithAndAlphanumeric("vcn_"),
		stringvalidator.LengthBetween(3, 128),
	)
}

func ValidSchemaRegistryName() validator.String {
	return stringvalidator.All(
		StartsWithAndAlphanumeric("vcn_sr_"),
		stringvalidator.LengthBetween(3, 128),
	)
}

func ValidTableFlowName() validator.String {
	return stringvalidator.All(
		StartsWithAndAlphanumeric("vcn_dl_"),
		stringvalidator.LengthBetween(3, 128),
	)
}

func alphaNumericSpaceesUnderscoresHyphensOnly() validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(`^[a-z_\-A-Z0-9 ]*$`),
		"must contain only alphanumeric characters, spaces, underscores, and hyphens",
	)
}

func ValidWorkspaceName() validator.String {
	return stringvalidator.All(
		stringvalidator.LengthBetween(3, 128),
		alphaNumericSpaceesUnderscoresHyphensOnly(),
	)
}

func ValidUserRoleName() validator.String {
	return stringvalidator.All(
		stringvalidator.LengthBetween(3, 60),
		alphaNumericSpaceesUnderscoresHyphensOnly(),
	)
}

func StartsWith(prefix string) validator.String {
	return stringvalidator.RegexMatches(
		regexp.MustCompile(fmt.Sprintf("^%s.+$", prefix)),
		fmt.Sprintf("must start with '%s'", prefix),
	)
}

// Unfortunately, Golang's regex doesn't support negative lookahead,
// so we can't do ^(?!prefix).
type notStartWithValidator struct {
	prefix string
}

func (validator notStartWithValidator) Description(_ context.Context) string {
	return fmt.Sprintf("value must not start with '%s'", validator.prefix)
}

func (validator notStartWithValidator) MarkdownDescription(ctx context.Context) string {
	return validator.Description(ctx)
}

func (v notStartWithValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	if strings.HasPrefix(value, v.prefix) {
		response.Diagnostics.AddError("invalid prefix", fmt.Sprintf("property must not start with: %s", v.prefix))
	}
}

func NotStartWith(prefix string) validator.String {
	return notStartWithValidator{prefix: prefix}
}

type aclsExclusionValidator struct{}

func (v aclsExclusionValidator) Description(ctx context.Context) string {
	return "Ensures that exactly one of enable_acls or enable_acl_shadowing is true."
}

func (v aclsExclusionValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v aclsExclusionValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	var enableACLs types.Bool
	var enableShadowing types.Bool

	// Fetch the two attributes relative to the current object (configuration)
	diags := req.Config.GetAttribute(ctx, req.Path.AtName("enable_acls"), &enableACLs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = req.Config.GetAttribute(ctx, req.Path.AtName("enable_acl_shadowing"), &enableShadowing)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If values are unknown, defer validation.
	if enableACLs.IsUnknown() || enableShadowing.IsUnknown() {
		return
	}

	// Convert to bool
	acl := enableACLs.ValueBool()
	shadow := enableShadowing.ValueBool()

	// INVALID: both true
	if acl && shadow {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid ACL Configuration",
			fmt.Sprintf(
				"enable_acls and enable_acl_shadowing cannot both be true. Received enable_acls=%t and enable_acl_shadowing=%t.",
				acl, shadow,
			),
		)
		return
	}
}

func ACLModeMutualExclusion() validator.Object {
	return aclsExclusionValidator{}
}

// billingGrantValidator ensures that:
// 1. If grant_type is "billing", workspace_id must be "-"
// 2. If workspace_id is "-", grant_type must be "billing"
type billingGrantValidator struct{}

func (v billingGrantValidator) Description(ctx context.Context) string {
	return "Ensures that billing grant type is always associated with the empty workspace ID '-' and vice versa."
}

func (v billingGrantValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v billingGrantValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, resp *validator.ObjectResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	attrs := req.ConfigValue.Attributes()

	workspaceIDAttr, ok := attrs["workspace_id"]
	if !ok {
		return
	}
	workspaceID, ok := workspaceIDAttr.(types.String)
	if !ok || workspaceID.IsUnknown() {
		return
	}

	grantTypeAttr, ok := attrs["grant_type"]
	if !ok {
		return
	}
	grantType, ok := grantTypeAttr.(types.String)
	if !ok || grantType.IsUnknown() {
		return
	}

	wsID := workspaceID.ValueString()
	gt := grantType.ValueString()

	// If grant_type is "billing", workspace_id must be "-"
	if gt == "billing" && wsID != "-" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Billing Grant Configuration",
			fmt.Sprintf(
				"The 'billing' grant type must be assigned with the empty workspace_id '-'. Received workspace_id=%q.",
				wsID,
			),
		)
		return
	}

	// If workspace_id is "-", grant_type must be "billing"
	if wsID == "-" && gt != "billing" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Billing Grant Configuration",
			fmt.Sprintf(
				"The empty workspace ID '-' can only be assigned with the 'billing' grant type. Received grant_type=%q.",
				gt,
			),
		)
		return
	}
}

func BillingGrantConstraint() validator.Object {
	return billingGrantValidator{}
}
