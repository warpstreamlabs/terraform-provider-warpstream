package provider

import (
	"context"
	"fmt"

	"github.com/kylelemons/godebug/diff"

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
	VirtualClusterID             types.String                 `tfsdk:"virtual_cluster_id"`
	ID                           types.String                 `tfsdk:"id"`
	Name                         types.String                 `tfsdk:"name"`
	State                        types.String                 `tfsdk:"state"`
	Type                         types.String                 `tfsdk:"type"`
	DeployedConfigurationVersion types.Int64                  `tfsdk:"deployed_configuration_version"`
	Configurations               []pipelineConfigurationModel `tfsdk:"configurations"`
}

// pipelineConfigurationModel maps credentials schema data.
type pipelineConfigurationModel struct {
	ID                types.String    `tfsdk:"id"`
	Version           types.Int64     `tfsdk:"version"`
	ConfigurationYAML utils.YamlValue `tfsdk:"configuration_yaml"`
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
				Description: "The name of the pipeline.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Description: "Pipeline ID.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "Pipeline state: 'running' 'paused'.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("running", "paused"),
				},
			},
			"type": schema.StringAttribute{
				Description: "Pipeline type",
				Computed:    true,
			},
			"deployed_configuration_version": schema.Int64Attribute{
				Description: "Deployed configuration version.",
				Required:    true,
			},
			"configurations": schema.ListNestedAttribute{
				Description: "List of immutable Configurations",
				Required:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Pipeline Configuration ID.",
							Computed:    true,
						},
						"version": schema.Int64Attribute{
							Description: "Monotonically Increasing Version of the Configuration.",
							Required:    true,
							Validators:  []validator.Int64{},
						},
						"configuration_yaml": schema.StringAttribute{
							Description: "The YAML configuration for the pipeline.",
							Required:    true,
							CustomType:  utils.YamlType{},
						},
					},
				},
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

	if len(plan.Configurations) == 0 {
		resp.Diagnostics.AddError(
			"Pipeline Configuration Missing",
			"A WarpStream pipeline requires at least one configuration. Please add a `configurations` block to your resource definition.",
		)
		return
	}
	if int(plan.DeployedConfigurationVersion.ValueInt64()) >= len(plan.Configurations) {
		resp.Diagnostics.AddError(
			"Invalid Deployed Configuration Version",
			fmt.Sprintf("The `deployed_configuration_version` (%d) cannot be greater than or equal to the number of configurations (%d). Please ensure the deployed version is valid.",
				plan.DeployedConfigurationVersion.ValueInt64(), len(plan.Configurations)),
		)
		return
	}

	c, err := r.client.CreatePipeline(ctx, api.HTTPCreatePipelineRequest{
		VirtualClusterID: plan.VirtualClusterID.ValueString(),
		PipelineName:     plan.Name.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating WarpStream Pipeline Configuration",
			fmt.Sprintf("Could not create the pipeline configuration. Please check your configuration and try again. Details: %s", err.Error()),
		)
		return
	}

	resultPlan := pipelineModel{
		VirtualClusterID: plan.VirtualClusterID,
		ID:               types.StringValue(c.PipelineID),
		Name:             plan.Name,
		Type:             types.StringValue(c.PipelineType),
		// State and DeployedConfigurationVersion are set later.
		Configurations: []pipelineConfigurationModel{},
	}

	for version, conf := range plan.Configurations {
		if version != int(conf.Version.ValueInt64()) {
			resp.Diagnostics.AddError(
				"Configuration Version Mismatch",
				fmt.Sprintf("The version of configuration at index %d should be %d. Please ensure the versions are sequential starting from 0.", version, version),
			)
			return
		}

		cc, err := r.client.CreatePipelineConfiguration(ctx, api.HTTPCreatePipelineConfigurationRequest{
			VirtualClusterID:  plan.VirtualClusterID.ValueString(),
			PipelineID:        c.PipelineID,
			ConfigurationYAML: conf.ConfigurationYAML.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating WarpStream Pipeline Configuration",
				"Could not create WarpStream Pipeline Configuration, unexpected error: "+err.Error(),
			)
			return
		}

		resultConf := pipelineConfigurationModel{
			ID:                types.StringValue(cc.ConfigurationID),
			Version:           types.Int64Value(int64(version)),
			ConfigurationYAML: conf.ConfigurationYAML,
		}
		resultPlan.Configurations = append(resultPlan.Configurations, resultConf)

		if version == int(plan.DeployedConfigurationVersion.ValueInt64()) {
			_, err := r.client.ChangePipelineState(ctx, api.HTTPChangePipelineStateRequest{
				VirtualClusterID:        plan.VirtualClusterID.ValueString(),
				PipelineID:              resultPlan.ID.ValueString(),
				DesiredState:            plan.State.ValueStringPointer(),
				DeployedConfigurationID: resultConf.ID.ValueStringPointer(),
			})
			if err != nil {
				resp.Diagnostics.AddError(
					"Error setting WarpStream Pipeline state",
					"Could not set WarpStream Pipeline state, unexpected error: "+err.Error(),
				)
				return
			}
			resultPlan.DeployedConfigurationVersion = plan.DeployedConfigurationVersion
			resultPlan.State = plan.State
		}
	}

	plan = resultPlan

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
		Type:             types.StringValue(pipeline.PipelineOverview.Type),
		// TODO: DeployedConfigurationVersion: ,
		Configurations: []pipelineConfigurationModel{},
	}

	for version, conf := range pipeline.Configurations {
		confResult := pipelineConfigurationModel{
			ID:                types.StringValue(conf.ID),
			Version:           types.Int64Value(int64(conf.Version)),
			ConfigurationYAML: utils.StringToYaml(conf.ConfigurationYAML),
		}
		state.Configurations = append(state.Configurations, confResult)

		if conf.ID == pipeline.PipelineOverview.DeployedConfigurationId {
			state.DeployedConfigurationVersion = types.Int64Value(int64(version))
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

	for version, conf := range state.Configurations {
		if len(plan.Configurations) <= version {
			resp.Diagnostics.AddError(
				"Configuration Removal Not Allowed",
				fmt.Sprintf("Configuration at version %d cannot be removed. Configurations are immutable in WarpStream pipelines. You can only append new configurations.", version),
			)
			return
		}
		prevConf := plan.Configurations[version]
		prevYaml, _ := utils.NormalizeYAML(prevConf.ConfigurationYAML.ValueString())
		newYaml, _ := utils.NormalizeYAML(conf.ConfigurationYAML.ValueString())
		if newYaml != prevYaml {
			// TODO: Print diff.
			diffStr := diff.Diff(prevYaml, newYaml)
			resp.Diagnostics.AddError(
				"Immutable Configuration Modified",
				fmt.Sprintf(
					"The YAML configuration for version %d has been modified. "+
						"Configurations are immutable and can only be appended. Consider creating a new version instead.\n\nDifference:\n%s",
					version, diffStr,
				),
			)
			return
		}
	}

	if int(plan.DeployedConfigurationVersion.ValueInt64()) >= len(plan.Configurations) {
		resp.Diagnostics.AddError(
			"Invalid Deployed Configuration Version",
			fmt.Sprintf("The `deployed_configuration_version` (%d) cannot be greater than or equal to the number of configurations (%d). Please ensure the deployed version is valid.",
				plan.DeployedConfigurationVersion.ValueInt64(), len(plan.Configurations)),
		)
		return
	}

	deployedHasChanged := plan.DeployedConfigurationVersion != state.DeployedConfigurationVersion ||
		plan.State != state.State
	for version, conf := range plan.Configurations {
		if version != int(conf.Version.ValueInt64()) {
			resp.Diagnostics.AddError(
				"Configuration Version Mismatch",
				fmt.Sprintf("The version of configuration at index %d should be %d. Please ensure the versions are sequential starting from 0.", version, version),
			)
			return
		}

		if len(state.Configurations) <= version {
			cc, err := r.client.CreatePipelineConfiguration(ctx, api.HTTPCreatePipelineConfigurationRequest{
				VirtualClusterID:  plan.VirtualClusterID.ValueString(),
				PipelineID:        state.ID.ValueString(),
				ConfigurationYAML: conf.ConfigurationYAML.ValueString(),
			})
			if err != nil {
				resp.Diagnostics.AddError(
					"Error creating WarpStream Pipeline Configuration",
					"Could not create WarpStream Pipeline Configuration, unexpected error: "+err.Error(),
				)
				return
			}

			resultConf := pipelineConfigurationModel{
				ID:                types.StringValue(cc.ConfigurationID),
				Version:           types.Int64Value(int64(version)),
				ConfigurationYAML: conf.ConfigurationYAML,
			}
			state.Configurations = append(state.Configurations, resultConf)
		}

		if deployedHasChanged && version == int(plan.DeployedConfigurationVersion.ValueInt64()) {
			_, err := r.client.ChangePipelineState(ctx, api.HTTPChangePipelineStateRequest{
				VirtualClusterID:        plan.VirtualClusterID.ValueString(),
				PipelineID:              state.ID.ValueString(),
				DesiredState:            plan.State.ValueStringPointer(),
				DeployedConfigurationID: state.Configurations[version].ID.ValueStringPointer(),
			})
			if err != nil {
				resp.Diagnostics.AddError(
					"Error setting WarpStream Pipeline state",
					"Could not set WarpStream Pipeline state, unexpected error: "+err.Error(),
				)
				return
			}
			state.DeployedConfigurationVersion = plan.DeployedConfigurationVersion
			state.State = plan.State
		}
	}
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
