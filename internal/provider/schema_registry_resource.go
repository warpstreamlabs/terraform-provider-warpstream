package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	warpstreamtypes "github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &schemaRegistryResource{}
	_ resource.ResourceWithConfigure   = &schemaRegistryResource{}
	_ resource.ResourceWithImportState = &schemaRegistryResource{}
)

type schemaRegistryResource struct {
	client *api.Client
}

func NewSchemaRegistryResource() resource.Resource {
	return &schemaRegistryResource{}
}

func (r *schemaRegistryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_schema_registry"
}

func (r *schemaRegistryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

var registryCloudSchema = schema.SingleNestedAttribute{
	Attributes: map[string]schema.Attribute{
		"provider": schema.StringAttribute{
			Description: "Cloud Provider. Valid providers are: `aws` (default) and `gcp`.",
			Computed:    true,
			Optional:    true,
			Default:     stringdefault.StaticString("aws"),
			Validators: []validator.String{
				stringvalidator.OneOf("aws", "gcp"),
			},
		},
		"region": schema.StringAttribute{
			Description: "Cloud Region. Defaults to `us-east-1`.",
			Computed:    true,
			Optional:    true,
			Default:     stringdefault.StaticString("us-east-1"),
		},
	},
	Description: "Virtual Cluster Cloud Location.",
	Optional:    true,
	Computed:    true,
	Default: objectdefault.StaticValue(
		types.ObjectValueMust(
			virtualClusterRegistryCloudModel{}.AttributeTypes(),
			virtualClusterRegistryCloudModel{}.DefaultObject(),
		)),
	PlanModifiers: []planmodifier.Object{
		objectplanmodifier.RequiresReplace(),
	},
}

func (r *schemaRegistryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create, update and delete schema registries.
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Schema Registry ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Schema Registry Name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.StartsWith("vcn_")},
			},
			"agent_keys": schema.ListNestedAttribute{
				Description:  "List of keys to authenticate an agent with this cluster.",
				Computed:     true,
				NestedObject: agentKeyResourceSchema,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "Virtual Cluster Creation Timestamp.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud": registryCloudSchema,
			"bootstrap_url": schema.StringAttribute{
				Description: "Bootstrap URL to connect to the Schema Registry.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *schemaRegistryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan schemaRegistryResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cloudPlan virtualClusterRegistryCloudModel
	diags = plan.Cloud.As(ctx, &cloudPlan, basetypes.ObjectAsOptions{})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new virtual cluster
	cluster, err := r.client.CreateVirtualCluster(
		plan.Name.ValueString(),
		api.ClusterParameters{
			Type:   warpstreamtypes.VirtualClusterTypeSchemaRegistry,
			Region: cloudPlan.Region.ValueStringPointer(),
			Cloud:  cloudPlan.Provider.ValueString(),
		})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating WarpStream Schema Registry",
			fmt.Sprintf("Could not create WarpStream Schema Registry Virtual Cluster, unexpected error: %v", err),
		)
		return
	}

	cluster, err = r.client.GetVirtualCluster(cluster.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster",
			fmt.Sprintf("Could not get Virtual Cluster %s: %v", cluster.ID, err),
		)
		return
	}

	state := schemaRegistryResourceModel{
		ID:        types.StringValue(cluster.ID),
		Name:      types.StringValue(cluster.Name),
		AgentKeys: plan.AgentKeys,
		CreatedAt: types.StringValue(cluster.CreatedAt),
		Cloud:     plan.Cloud,
	}

	if cluster.BootstrapURL != nil {
		state.BootstrapURL = types.StringValue(*cluster.BootstrapURL)
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentKeysState, ok := mapToAgentKeyModels(cluster.AgentKeys, &resp.Diagnostics)
	if !ok { // Diagnostics handled by helper.
		return
	}

	diags = resp.State.SetAttribute(ctx, path.Root("agent_keys"), agentKeysState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *schemaRegistryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state schemaRegistryResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cluster *api.VirtualCluster

	// Crossplane.io creates terraform state manually with empty IDs. There is
	// no terraform standard to handle empty IDs and our API does not handle
	// them in a way that is useful. Other TF providers are a mixed bag when
	// handling empty IDs, so let's explicitly handle them.
	if state.ID.ValueString() == "" {
		var err error
		cluster, err = r.client.FindVirtualCluster(state.Name.ValueString())
		if err != nil {
			if errors.Is(err, api.ErrNotFound) {
				resp.State.RemoveResource(ctx)
				return
			}

			resp.Diagnostics.AddError(
				"Error Reading WarpStream Virtual Cluster",
				"Could not read WarpStream Virtual Cluster Name "+state.Name.ValueString()+": "+err.Error(),
			)
		}
		state.ID = types.StringValue(cluster.ID)
	}

	cluster, err := r.client.GetVirtualCluster(state.ID.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading WarpStream Virtual Cluster",
			"Could not read WarpStream Virtual Cluster ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.ID = types.StringValue(cluster.ID)
	state.Name = types.StringValue(cluster.Name)
	state.CreatedAt = types.StringValue(cluster.CreatedAt)
	if cluster.BootstrapURL != nil {
		state.BootstrapURL = types.StringValue(*cluster.BootstrapURL)
	}

	cloudValue, diagnostics := types.ObjectValue(
		virtualClusterRegistryCloudModel{}.AttributeTypes(),
		map[string]attr.Value{
			"provider": types.StringValue(cluster.CloudProvider),
			// schema registry is always single region
			"region": types.StringValue(cluster.ClusterRegion.Region.Name),
		},
	)
	if diagnostics != nil {
		resp.Diagnostics.Append(diagnostics...)
		return
	}
	state.Cloud = cloudValue

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentKeysState, ok := mapToAgentKeyModels(cluster.AgentKeys, &resp.Diagnostics)
	if !ok { // Diagnostics handled by helper.
		return
	}
	diags = resp.State.SetAttribute(ctx, path.Root("agent_keys"), agentKeysState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *schemaRegistryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan virtualClusterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *schemaRegistryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state schemaRegistryResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteVirtualCluster(state.ID.ValueString(), state.Name.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting WarpStream Schema Registry",
			fmt.Sprintf("Could not delete WarpStream Schema Registry %s: %v", state.Name, err),
		)
		return
	}
}

func (r *schemaRegistryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
