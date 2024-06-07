package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &pipelineResource{}
	_ resource.ResourceWithConfigure = &pipelineResource{}
)

// NewPipelineResource is a helper function to simplify the provider implementation.
func NewPipelineResource() resource.Resource {
	return &pipelineResource{}
}

// pipelineResource is the resource implementation.
type pipelineResource struct {
	client *api.Client
}

// pipelineModel maps credentials schema data.
type pipelineModel struct {
	VirtualClusterID  types.String    `tfsdk:"virtual_cluster_id"`
	ID                types.String    `tfsdk:"id"`
	Name              types.String    `tfsdk:"name"`
	State             types.String    `tfsdk:"state"`
	ConfigurationYAML utils.YamlValue `tfsdk:"configuration_yaml"`
	ConfigurationID   types.String    `tfsdk:"configuration_id"`
}

// Configure adds the provider configured client to the data source.
func (r *pipelineResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Provider Configuration",
			fmt.Sprintf("Expected an API Client instance, but got: %T. Please ensure the provider is configured correctly.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Metadata returns the resource type name.
func (r *pipelineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline"
}

// Schema defines the schema for the resource.
func (r *pipelineResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `
This resource allows you to create pipelines.
For more details, take a look at: https://docs.warpstream.com/warpstream/configuration/benthos
`,
		Attributes: map[string]schema.Attribute{
			"virtual_cluster_id": schema.StringAttribute{
				Description: "The ID of the virtual cluster associated with the pipeline.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The unique human-readable name of the pipeline within the virtual cluster. This cannot be changed after creation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier of the pipeline, automatically generated by WarpStream upon creation.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "The desired operational state of the pipeline. Valid values are 'running' or 'paused'.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("running", "paused"),
				},
			},
			"configuration_yaml": schema.StringAttribute{
				Description: "The YAML content defining the input sources, processing steps, and output destinations for the pipeline. " +
					"This represents the complete configuration for this specific version. To understand how to set your configuration take a look at: https://docs.warpstream.com/warpstream/configuration/benthos#getting-started",
				Required:   true,
				CustomType: utils.YamlType{},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"configuration_id": schema.StringAttribute{
				Description: "The unique identifier of the pipeline configuration, automatically generated by WarpStream upon creation.",
				Computed:    true,
			},
		},
	}
}

// Create a new resource.
func (r *pipelineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan pipelineModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := r.client.CreatePipeline(ctx, api.HTTPCreatePipelineRequest{
		VirtualClusterID: plan.VirtualClusterID.ValueString(),
		PipelineName:     plan.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Pipeline Creation Failed",
			fmt.Sprintf("Failed to create pipeline '%s' in virtual cluster '%s': %s", plan.Name.ValueString(), plan.VirtualClusterID.ValueString(), err),
		)
		return
	}
	plan.ID = types.StringValue(c.PipelineID)

	cc, err := r.client.CreatePipelineConfiguration(ctx, api.HTTPCreatePipelineConfigurationRequest{
		VirtualClusterID:  plan.VirtualClusterID.ValueString(),
		PipelineID:        c.PipelineID,
		ConfigurationYAML: plan.ConfigurationYAML.ValueString(),
	})
	if err != nil {
		r.client.DeletePipeline(ctx, api.HTTPDeletePipelineRequest{
			VirtualClusterID: plan.VirtualClusterID.ValueString(),
			PipelineID:       plan.ID.ValueString(),
		})
		resp.Diagnostics.AddError(
			"Error creating WarpStream Pipeline Configuration",
			"Could not create WarpStream Pipeline Configuration, unexpected error: "+err.Error(),
		)
		return
	}
	plan.ConfigurationID = types.StringValue(cc.ConfigurationID)

	_, err = r.client.ChangePipelineState(ctx, api.HTTPChangePipelineStateRequest{
		VirtualClusterID:        plan.VirtualClusterID.ValueString(),
		PipelineID:              plan.ID.ValueString(),
		DesiredState:            plan.State.ValueStringPointer(),
		DeployedConfigurationID: plan.ConfigurationID.ValueStringPointer(),
	})
	if err != nil {
		r.client.DeletePipeline(ctx, api.HTTPDeletePipelineRequest{
			VirtualClusterID: plan.VirtualClusterID.ValueString(),
			PipelineID:       plan.ID.ValueString(),
		})
		resp.Diagnostics.AddError(
			"Error setting WarpStream Pipeline state",
			"Could not set WarpStream Pipeline state, unexpected error: "+err.Error(),
		)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *pipelineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pipelineModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	pipeline, err := r.client.DescribePipeline(ctx, api.HTTPDescribePipelineRequest{
		VirtualClusterID: state.VirtualClusterID.ValueString(),
		PipelineID:       state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Pipeline",
			fmt.Sprintf("Unable to fetch details for pipeline '%s'. Please check the pipeline ID and ensure it exists. Error details: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	state = pipelineModel{
		VirtualClusterID: state.VirtualClusterID,
		ID:               types.StringValue(pipeline.PipelineOverview.ID),
		Name:             types.StringValue(pipeline.PipelineOverview.Name),
		State:            types.StringValue(pipeline.PipelineOverview.State),
	}

	for _, conf := range pipeline.Configurations {
		if conf.ID == pipeline.PipelineOverview.DeployedConfigurationId {
			state.ConfigurationYAML = utils.StringToYaml(conf.ConfigurationYAML)
			state.ConfigurationID = types.StringValue(conf.ID)
		}
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *pipelineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan pipelineModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state pipelineModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deployedStateHasChanged := plan.State != state.State
	if !deployedStateHasChanged {
		return
	}

	_, err := r.client.ChangePipelineState(ctx, api.HTTPChangePipelineStateRequest{
		VirtualClusterID:        plan.VirtualClusterID.ValueString(),
		PipelineID:              state.ID.ValueString(),
		DesiredState:            plan.State.ValueStringPointer(),
		DeployedConfigurationID: state.ConfigurationID.ValueStringPointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error setting WarpStream Pipeline state",
			"Could not set WarpStream Pipeline state, unexpected error: "+err.Error(),
		)
		return
	}
	state.State = plan.State

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *pipelineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state pipelineModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeletePipeline(ctx, api.HTTPDeletePipelineRequest{
		VirtualClusterID: state.VirtualClusterID.ValueString(),
		PipelineID:       state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Pipeline",
			fmt.Sprintf("Unable to delete pipeline '%s'. Please check your permissions and ensure there are no dependencies on this pipeline. Error details: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}
