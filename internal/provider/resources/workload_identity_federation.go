package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &workloadIdentityFederationResource{}
	_ resource.ResourceWithConfigure   = &workloadIdentityFederationResource{}
	_ resource.ResourceWithImportState = &workloadIdentityFederationResource{}
)

// NewWorkloadIdentityFederationResource is a helper function to simplify the provider implementation.
func NewWorkloadIdentityFederationResource() resource.Resource {
	return &workloadIdentityFederationResource{}
}

type workloadIdentityFederationResource struct {
	client *api.Client
}

func (r *workloadIdentityFederationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *api.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *workloadIdentityFederationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_identity_federation"
}

func (r *workloadIdentityFederationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create and delete workload identity federation bindings.

A binding lets a workload authenticate to the control plane by presenting an OIDC token from an
external issuer instead of a long-lived agent key. Bindings are immutable; any change replaces the
binding.

The WarpStream provider must be authenticated with an application key to consume this resource.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Workload Identity Federation ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"virtual_cluster_id": schema.StringAttribute{
				Description: "Virtual Cluster ID the binding grants access to. Cannot be changed after creation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.StartsWithAndAlphanumeric("vci_")},
			},
			"name": schema.StringAttribute{
				Description: "Human-readable name for the binding, unique within the virtual cluster. Cannot be changed after creation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{stringvalidator.LengthBetween(3, 128)},
			},
			"issuer_url": schema.StringAttribute{
				Description: "HTTPS URL of the OIDC issuer whose tokens this binding accepts. Cannot be changed after creation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.StartsWith("https://")},
			},
			"claim_match_rules": schema.ListNestedAttribute{
				Description: "Claim match rules, all of which must match for a token to be accepted. At least one rule is required so a valid-but-unmatched token is never granted access. Cannot be changed after creation.",
				Required:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"claim_path": schema.StringAttribute{
							Description: "Dot-separated path into the token's claims (e.g. \"sub\"). A segment containing a literal dot can be double-quoted.",
							Required:    true,
							Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
						},
						"expected_value": schema.StringAttribute{
							Description: "Expected value for the claim, matched case-sensitively. A single trailing '*' acts as a prefix wildcard.",
							Required:    true,
							Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
						},
					},
				},
			},
			"read_only": schema.BoolAttribute{
				Description: "Whether credentials minted via this binding are read-only. Cannot be changed after creation.",
				Optional:    true,
				// Computed so a binding created without this field set doesn't show drift.
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"max_credential_ttl_seconds": schema.Int64Attribute{
				Description: "Maximum lifetime, in seconds, of a credential minted via this binding. Must be between 60 and 86400 (24h). Cannot be changed after creation.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Validators: []validator.Int64{int64validator.Between(60, 86400)},
			},
			"audience": schema.StringAttribute{
				Description: "OIDC audience the agent must request. Derived from the virtual cluster ID by the control plane, not configurable.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Binding creation timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *workloadIdentityFederationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan models.WorkloadIdentityFederation
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateWorkloadIdentityFederation(api.WorkloadIdentityFederation{
		VirtualClusterID:        plan.VirtualClusterID.ValueString(),
		Name:                    plan.Name.ValueString(),
		IssuerURL:               plan.IssuerURL.ValueString(),
		ClaimMatchRules:         plan.ToAPIClaimMatchRules(),
		ReadOnly:                plan.ReadOnly.ValueBool(),
		MaxCredentialTTLSeconds: plan.MaxCredentialTTLSeconds.ValueInt64(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Workload Identity Federation",
			"Could not create WarpStream Workload Identity Federation, unexpected error: "+err.Error(),
		)
		return
	}

	// Re-read so state reflects the persisted values (e.g. created_at at the database's timestamp
	// precision) rather than the create response, keeping state stable across refreshes and imports.
	fed, err := r.client.GetWorkloadIdentityFederation(created.VirtualClusterID, created.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Workload Identity Federation",
			"Could not read WarpStream Workload Identity Federation ID "+created.ID+": "+err.Error(),
		)
		return
	}

	state := models.MapToWorkloadIdentityFederation(fed)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *workloadIdentityFederationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.WorkloadIdentityFederation
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fed, err := r.client.GetWorkloadIdentityFederation(state.VirtualClusterID.ValueString(), state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading WarpStream Workload Identity Federation",
			"Could not read WarpStream Workload Identity Federation ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state = models.MapToWorkloadIdentityFederation(fed)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is a no-op: bindings are immutable, so every attribute triggers a replace. It exists only to
// satisfy resource.Resource.
func (r *workloadIdentityFederationResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

// ImportState imports a binding into Terraform state. Because a binding is identified by both its
// virtual cluster and its own ID, the import ID is the composite: virtual_cluster_id/federation_id.
// The remaining attributes are populated by the subsequent Read.
func (r *workloadIdentityFederationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected an ID in the format: virtual_cluster_id/workload_identity_federation_id",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("virtual_cluster_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *workloadIdentityFederationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state models.WorkloadIdentityFederation
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteWorkloadIdentityFederation(state.VirtualClusterID.ValueString(), state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Workload Identity Federation",
			"Could not delete WarpStream Workload Identity Federation, unexpected error: "+err.Error(),
		)
		return
	}
}
