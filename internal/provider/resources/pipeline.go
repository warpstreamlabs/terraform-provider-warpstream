package resources

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                   = &pipelineResource{}
	_ resource.ResourceWithConfigure      = &pipelineResource{}
	_ resource.ResourceWithImportState    = &pipelineResource{}
	_ resource.ResourceWithValidateConfig = &pipelineResource{}
)

type pipelineType = string

const (
	BentoPipelineType         pipelineType = "bento"
	OrbitPipelineType         pipelineType = "orbit"
	SchemaLinkingPipelineType pipelineType = "schema_linking"
	TableflowPipelineType     pipelineType = "tableflow"
)

// NewPipelineResource is a helper function to simplify the provider implementation.
func NewPipelineResource() resource.Resource {
	return &pipelineResource{}
}

// pipelineResource is the resource implementation.
type pipelineResource struct {
	client *api.Client
}

// parsePipelineID parses the ID which can be either:
// - Composite format: "virtual_cluster_id/pipeline_id" (new format)
// - Legacy format: just "pipeline_id" (old format, requires fallbackVirtualClusterID)
// Returns the virtual cluster ID and pipeline ID.
func parsePipelineID(id, fallbackVirtualClusterID string) (virtualClusterID, pipelineID string) {
	parts := strings.Split(id, "/")
	if len(parts) == 2 {
		// New composite format
		return parts[0], parts[1]
	}
	// Legacy format - use the fallback virtual cluster ID from state
	return fallbackVirtualClusterID, id
}

// pipelineModel maps credentials schema data.
type pipelineModel struct {
	VirtualClusterID    types.String    `tfsdk:"virtual_cluster_id"`
	ID                  types.String    `tfsdk:"id"`
	Name                types.String    `tfsdk:"name"`
	State               types.String    `tfsdk:"state"`
	ConfigurationYAML   utils.YamlValue `tfsdk:"configuration_yaml"`
	ConfigurationInputs types.Map       `tfsdk:"configuration_inputs"`
	ConfigurationID     types.String    `tfsdk:"configuration_id"`
	Type                types.String    `tfsdk:"type"`
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
This resource allows you to create pipelines (Bento, Orbit, and Schema Linking).
For more details, take a look at: https://docs.warpstream.com/warpstream/configuration/bento, https://docs.warpstream.com/warpstream/byoc/orbit and https://docs.warpstream.com/warpstream/byoc/schema-registry/warpstream-schema-linking.

The WarpStream provider must be authenticated with an application key to consume this resource.
`,
		Attributes: map[string]schema.Attribute{
			"virtual_cluster_id": schema.StringAttribute{
				Description: "The ID of the virtual cluster associated with the pipeline.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.StartsWithAndAlphanumeric("vci_")},
			},
			"name": schema.StringAttribute{
				Description: "The unique human-readable name of the pipeline within the virtual cluster. This cannot be changed after creation.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The unique identifier of the pipeline, automatically generated by WarpStream upon creation. Format is 'virtual_cluster_id/pipeline_id'",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Description: "The desired operational state of the pipeline. Valid values are 'running' or 'paused'.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf("running", "paused"),
				},
			},
			"configuration_yaml": schema.StringAttribute{
				Description: "The YAML content defining the complete pipeline configuration. " +
					"Mutually exclusive with configuration_inputs. " +
					"Required for non-tableflow pipeline types.",
				Optional:   true,
				CustomType: utils.YamlType{},
			},
			"configuration_inputs": schema.MapAttribute{
				Description: "A map of named YAML configuration parts that are merged server-side into a single configuration. " +
					"Map keys are part names (supporting '/' for tree hierarchy, e.g. 'analytics/tables'). " +
					"Map values are YAML strings. Only supported for tableflow pipelines. " +
					"Mutually exclusive with configuration_yaml.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					utils.YamlMapSemanticEquals(),
				},
			},
			"configuration_id": schema.StringAttribute{
				Description: "The unique identifier of the pipeline configuration, automatically generated by WarpStream upon creation.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					utils.ConfigurationIDUseStateForUnchanged(),
				},
			},
			"type": schema.StringAttribute{
				Description: "Pipeline type. Valid types are: `bento` (default), `orbit`, `schema_linking`, `tableflow`",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(BentoPipelineType),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(BentoPipelineType, OrbitPipelineType, SchemaLinkingPipelineType, TableflowPipelineType),
				},
			},
		},
	}
}

func (r *pipelineResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config pipelineModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasYAML := !config.ConfigurationYAML.IsNull() && !config.ConfigurationYAML.IsUnknown()
	hasInputs := !config.ConfigurationInputs.IsNull() && !config.ConfigurationInputs.IsUnknown()

	if hasYAML && hasInputs {
		resp.Diagnostics.AddError(
			"Invalid Pipeline Configuration",
			"Cannot specify both configuration_yaml and configuration_inputs. Use one or the other.",
		)
		return
	}

	if !hasYAML && !hasInputs {
		resp.Diagnostics.AddError(
			"Missing Pipeline Configuration",
			"Either configuration_yaml or configuration_inputs must be provided.",
		)
		return
	}

	if hasInputs {
		pipelineType := config.Type.ValueString()
		// Type defaults to "bento" when not set, but during validation it may be unknown.
		if !config.Type.IsNull() && !config.Type.IsUnknown() && pipelineType != TableflowPipelineType {
			resp.Diagnostics.AddError(
				"Invalid Pipeline Configuration",
				"configuration_inputs is only supported for tableflow pipelines. Use configuration_yaml instead.",
			)
			return
		}

		for key, elem := range config.ConfigurationInputs.Elements() {
			strVal, ok := elem.(types.String)
			if !ok || strVal.IsNull() || strVal.IsUnknown() {
				continue
			}
			if _, err := utils.NormalizeYAML(strVal.ValueString()); err != nil {
				resp.Diagnostics.AddAttributeError(
					path.Root("configuration_inputs").AtMapKey(key),
					"Invalid YAML Value",
					fmt.Sprintf("The value for key %q is not valid YAML: %s", key, err.Error()),
				)
			}
		}
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
		Type:             plan.Type.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Pipeline Creation Failed",
			fmt.Sprintf("Failed to create pipeline '%s' in virtual cluster '%s': %s", plan.Name.ValueString(), plan.VirtualClusterID.ValueString(), err),
		)
		return
	}
	generatedId := fmt.Sprintf("%s/%s", plan.VirtualClusterID.ValueString(), c.PipelineID)
	plan.ID = types.StringValue(generatedId)

	configReq := api.HTTPCreatePipelineConfigurationRequest{
		VirtualClusterID: plan.VirtualClusterID.ValueString(),
		PipelineID:       c.PipelineID,
	}
	if !plan.ConfigurationInputs.IsNull() {
		inputs, err := configInputsFromMap(ctx, plan.ConfigurationInputs)
		if err != nil {
			if _, resetErr := r.client.DeletePipeline(ctx, api.HTTPDeletePipelineRequest{
				VirtualClusterID: plan.VirtualClusterID.ValueString(),
				PipelineID:       c.PipelineID,
			}); resetErr != nil {
				resp.Diagnostics.AddError(
					"Error resetting WarpStream Pipeline state",
					"Pipeline creation failed, and an attempt to reset the state also failed. Original error: "+err.Error()+
						". Reset error: "+resetErr.Error(),
				)
			}
			resp.Diagnostics.AddError("Error building configuration inputs", err.Error())
			return
		}
		configReq.ConfigurationInputs = inputs
	} else {
		configReq.ConfigurationYAML = plan.ConfigurationYAML.ValueString()
	}

	cc, err := r.client.CreatePipelineConfiguration(ctx, configReq)
	if err != nil {
		if _, resetErr := r.client.DeletePipeline(ctx, api.HTTPDeletePipelineRequest{
			VirtualClusterID: plan.VirtualClusterID.ValueString(),
			PipelineID:       c.PipelineID,
		}); resetErr != nil {
			resp.Diagnostics.AddError(
				"Error resetting WarpStream Pipeline state",
				"Pipeline creation failed, and an attempt to reset the state also failed. Original error: "+err.Error()+
					". Reset error: "+resetErr.Error(),
			)
		}
		resp.Diagnostics.AddError(
			"Error creating WarpStream Pipeline Configuration",
			"Could not create WarpStream Pipeline Configuration, unexpected error: "+err.Error(),
		)
		return
	}
	plan.ConfigurationID = types.StringValue(cc.ConfigurationID)

	_, err = r.client.ChangePipelineState(ctx, api.HTTPChangePipelineStateRequest{
		VirtualClusterID:        plan.VirtualClusterID.ValueString(),
		PipelineID:              c.PipelineID,
		DesiredState:            plan.State.ValueStringPointer(),
		DeployedConfigurationID: plan.ConfigurationID.ValueStringPointer(),
	})
	if err != nil {
		if _, resetErr := r.client.DeletePipeline(ctx, api.HTTPDeletePipelineRequest{
			VirtualClusterID: plan.VirtualClusterID.ValueString(),
			PipelineID:       c.PipelineID,
		}); resetErr != nil {
			resp.Diagnostics.AddError(
				"Error resetting WarpStream Pipeline state",
				"Pipeline creation failed, and an attempt to reset the state also failed. Original error: "+err.Error()+
					". Reset error: "+resetErr.Error(),
			)
		}
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

	// Parse ID - handles both composite (new) and legacy formats
	virtualClusterID, pipelineID := parsePipelineID(state.ID.ValueString(), state.VirtualClusterID.ValueString())

	pipeline, err := r.client.DescribePipeline(ctx, api.HTTPDescribePipelineRequest{
		VirtualClusterID: virtualClusterID,
		PipelineID:       pipelineID,
	})
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading Pipeline",
			fmt.Sprintf("Unable to fetch details for pipeline '%s'. Please check the pipeline ID and ensure it exists. Error details: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}

	// Maintain the composite ID format: virtual_cluster_id/pipeline_id
	compositeID := fmt.Sprintf("%s/%s", virtualClusterID, pipeline.PipelineOverview.ID)

	// Preserve which configuration attribute the user declared.
	priorUsedInputs := !state.ConfigurationInputs.IsNull()

	state = pipelineModel{
		VirtualClusterID:    types.StringValue(virtualClusterID),
		ID:                  types.StringValue(compositeID),
		Name:                types.StringValue(pipeline.PipelineOverview.Name),
		State:               types.StringValue(pipeline.PipelineOverview.State),
		Type:                types.StringValue(pipeline.PipelineOverview.Type),
		ConfigurationYAML:   utils.YamlValue{StringValue: types.StringNull()},
		ConfigurationInputs: types.MapNull(types.StringType),
	}

	for _, conf := range pipeline.Configurations {
		if conf.ID == pipeline.PipelineOverview.DeployedConfigurationId {
			state.ConfigurationID = types.StringValue(conf.ID)

			if priorUsedInputs && len(conf.ConfigurationInputs) > 0 {
				inputsMap := make(map[string]attr.Value, len(conf.ConfigurationInputs))
				for _, input := range conf.ConfigurationInputs {
					normalized, err := utils.NormalizeYAML(input.Yaml)
					if err != nil {
						normalized = input.Yaml
					}
					inputsMap[input.Name] = types.StringValue(normalized)
				}
				mapValue, mapDiags := types.MapValue(types.StringType, inputsMap)
				resp.Diagnostics.Append(mapDiags...)
				if resp.Diagnostics.HasError() {
					return
				}
				state.ConfigurationInputs = mapValue
			} else {
				state.ConfigurationYAML = utils.StringToYaml(conf.ConfigurationYAML)
			}
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

	// Parse ID - handles both composite (new) and legacy formats
	_, pipelineID := parsePipelineID(state.ID.ValueString(), state.VirtualClusterID.ValueString())

	usingInputs := !plan.ConfigurationInputs.IsNull()
	var configHasChanged bool
	if usingInputs {
		configHasChanged = !plan.ConfigurationInputs.Equal(state.ConfigurationInputs)
	} else {
		configHasChanged = plan.ConfigurationYAML != state.ConfigurationYAML
	}

	desiredConfigurationID := state.ConfigurationID
	if configHasChanged {
		configReq := api.HTTPCreatePipelineConfigurationRequest{
			VirtualClusterID: plan.VirtualClusterID.ValueString(),
			PipelineID:       pipelineID,
		}
		if usingInputs {
			inputs, err := configInputsFromMap(ctx, plan.ConfigurationInputs)
			if err != nil {
				resp.Diagnostics.AddError("Error building configuration inputs", err.Error())
				return
			}
			configReq.ConfigurationInputs = inputs
		} else {
			configReq.ConfigurationYAML = plan.ConfigurationYAML.ValueString()
		}

		createConfigResp, err := r.client.CreatePipelineConfiguration(ctx, configReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating WarpStream Pipeline Configuration",
				"Could not create WarpStream Pipeline Configuration, unexpected error: "+err.Error(),
			)
			return
		}
		desiredConfigurationID = types.StringValue(createConfigResp.ConfigurationID)
	}

	deployedStateHasChanged := plan.State != state.State || desiredConfigurationID != state.ConfigurationID
	if deployedStateHasChanged {
		_, err := r.client.ChangePipelineState(ctx, api.HTTPChangePipelineStateRequest{
			VirtualClusterID:        plan.VirtualClusterID.ValueString(),
			PipelineID:              pipelineID,
			DesiredState:            plan.State.ValueStringPointer(),
			DeployedConfigurationID: desiredConfigurationID.ValueStringPointer(),
		})
		if err != nil {
			resp.Diagnostics.AddError(
				"Error setting WarpStream Pipeline state",
				"Could not set WarpStream Pipeline state, unexpected error: "+err.Error(),
			)
			return
		}
	}

	state.State = plan.State
	state.ConfigurationID = desiredConfigurationID
	state.ConfigurationYAML = plan.ConfigurationYAML
	state.ConfigurationInputs = plan.ConfigurationInputs

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

	// Parse ID - handles both composite (new) and legacy formats
	_, pipelineID := parsePipelineID(state.ID.ValueString(), state.VirtualClusterID.ValueString())

	_, err := r.client.DeletePipeline(ctx, api.HTTPDeletePipelineRequest{
		VirtualClusterID: state.VirtualClusterID.ValueString(),
		PipelineID:       pipelineID,
	})
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}

		resp.Diagnostics.AddError(
			"Error Deleting Pipeline",
			fmt.Sprintf("Unable to delete pipeline '%s'. Please check your permissions and ensure there are no dependencies on this pipeline. Error details: %s", state.ID.ValueString(), err.Error()),
		)
		return
	}
}

// configInputsFromMap converts a Terraform map of name->yaml into a sorted
// slice of HTTPConfigurationInput for the API.
func configInputsFromMap(ctx context.Context, inputsMap types.Map) ([]api.HTTPConfigurationInput, error) {
	var goMap map[string]string
	diags := inputsMap.ElementsAs(ctx, &goMap, false)
	if diags.HasError() {
		return nil, fmt.Errorf("error reading configuration_inputs: %s", diags.Errors()[0].Detail())
	}

	names := make([]string, 0, len(goMap))
	for name := range goMap {
		names = append(names, name)
	}
	sort.Strings(names)

	inputs := make([]api.HTTPConfigurationInput, len(names))
	for i, name := range names {
		inputs[i] = api.HTTPConfigurationInput{
			Name: name,
			Yaml: goMap[name],
		}
	}
	return inputs, nil
}

func (r *pipelineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected an ID in the format virtual_cluster_id/pipeline_id",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("virtual_cluster_id"), parts[0])...)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
