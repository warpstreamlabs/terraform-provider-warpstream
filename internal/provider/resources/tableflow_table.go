package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/utils"
)

var (
	_ resource.Resource                = &tableflowTableResource{}
	_ resource.ResourceWithConfigure   = &tableflowTableResource{}
	_ resource.ResourceWithImportState = &tableflowTableResource{}
)

func NewTableFlowTableResource() resource.Resource {
	return &tableflowTableResource{}
}

type tableflowTableResource struct {
	client *api.Client
}

type tableflowTableModel struct {
	VirtualClusterID  types.String `tfsdk:"virtual_cluster_id"`
	ID                types.String `tfsdk:"id"`
	TableName         types.String `tfsdk:"table_name"`
	RecreationKey     types.String `tfsdk:"recreation_key"`
	TableUUID         types.String `tfsdk:"table_uuid"`
	SourceStreamName  types.String `tfsdk:"source_stream_name"`
	SourceClusterName types.String `tfsdk:"source_cluster_name"`
	CreatedAt         types.String `tfsdk:"created_at"`
}

func (r *tableflowTableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*api.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Invalid Provider Configuration",
			fmt.Sprintf("Expected an API Client instance, but got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *tableflowTableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tableflow_table"
}

func (r *tableflowTableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `Manages a Tableflow table lifecycle. This resource adopts an existing table 
(created automatically by the Tableflow scheduler from the pipeline configuration) and allows 
programmatic recreation by changing the recreation_key attribute. When destroyed, the table is 
deleted via the API and the scheduler will automatically recreate it if the pipeline config 
still contains the table definition.`,
		Attributes: map[string]schema.Attribute{
			"virtual_cluster_id": schema.StringAttribute{
				Description: "The ID of the Tableflow virtual cluster.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{utils.StartsWithAndAlphanumeric("vci_")},
			},
			"table_name": schema.StringAttribute{
				Description: "The name of the Tableflow table to manage. Must match a table created by the pipeline scheduler.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"recreation_key": schema.StringAttribute{
				Description: "An arbitrary string whose change forces table recreation (destroy + create). " +
					"Bump this value (e.g. from \"v1\" to \"v2\") to trigger a safe table drop and re-creation by the scheduler.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Description: "Resource identifier in the format virtual_cluster_id/table_uuid.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"table_uuid": schema.StringAttribute{
				Description: "The UUID of the table, assigned by WarpStream.",
				Computed:    true,
			},
			"source_stream_name": schema.StringAttribute{
				Description: "The Kafka topic name that feeds this table.",
				Computed:    true,
			},
			"source_cluster_name": schema.StringAttribute{
				Description: "The name of the source cluster for this table.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The table creation timestamp (RFC3339).",
				Computed:    true,
			},
		},
	}
}

func (r *tableflowTableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tableflowTableModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	table, err := r.lookupTable(ctx, plan.VirtualClusterID.ValueString(), plan.TableName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Table Not Found",
			fmt.Sprintf(
				"Could not find Tableflow table %q in virtual cluster %q. "+
					"Ensure the pipeline configuration defines this table and the scheduler has created it. Error: %s",
				plan.TableName.ValueString(), plan.VirtualClusterID.ValueString(), err,
			),
		)
		return
	}

	populateTableModel(&plan, table)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *tableflowTableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tableflowTableModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	table, err := r.lookupTable(ctx, state.VirtualClusterID.ValueString(), state.TableName.ValueString())
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading Tableflow Table",
			fmt.Sprintf("Unable to read table %q: %s", state.TableName.ValueString(), err),
		)
		return
	}

	populateTableModel(&state, table)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *tableflowTableResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"All mutable attributes on warpstream_tableflow_table use RequiresReplace. Update should never be called.",
	)
}

func (r *tableflowTableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tableflowTableModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DLDeleteTable(ctx, api.DLDeleteTableRequest{
		VirtualClusterID: state.VirtualClusterID.ValueString(),
		TableUUID:        state.TableUUID.ValueString(),
	})
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return
		}
		resp.Diagnostics.AddError(
			"Error Deleting Tableflow Table",
			fmt.Sprintf("Unable to delete table %q (UUID %s): %s",
				state.TableName.ValueString(), state.TableUUID.ValueString(), err),
		)
	}
}

func (r *tableflowTableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Expected import ID in the format: virtual_cluster_id/table_name",
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("virtual_cluster_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table_name"), parts[1])...)
}

func (r *tableflowTableResource) lookupTable(ctx context.Context, virtualClusterID, tableName string) (*api.DLTable, error) {
	getResp, err := r.client.DLGetTable(ctx, api.DLGetTableRequest{
		VirtualClusterID: virtualClusterID,
		TableName:        tableName,
	})
	if err != nil {
		return nil, err
	}
	if getResp.Table == nil {
		return nil, fmt.Errorf("table %q not found: %w", tableName, api.ErrNotFound)
	}
	return getResp.Table, nil
}

func populateTableModel(model *tableflowTableModel, table *api.DLTable) {
	model.ID = types.StringValue(model.VirtualClusterID.ValueString() + "/" + table.TableUUID)
	model.TableUUID = types.StringValue(table.TableUUID)
	model.SourceStreamName = types.StringValue(table.SourceStreamName)
	model.SourceClusterName = types.StringValue(table.SourceClusterName)

	if table.CreatedAtUnixNanos > 0 {
		sec := int64(table.CreatedAtUnixNanos / 1_000_000_000)
		nsec := int64(table.CreatedAtUnixNanos % 1_000_000_000)
		model.CreatedAt = types.StringValue(
			time.Unix(sec, nsec).UTC().Format(time.RFC3339),
		)
	} else {
		model.CreatedAt = types.StringValue("")
	}
}
