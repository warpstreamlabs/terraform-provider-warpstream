package resources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

func TestEventsConfigured(t *testing.T) {
	t.Parallel()

	attrTypes := models.VirtualClusterEvents{}.AttributeTypes()
	present, presentDiags := types.ObjectValue(attrTypes, models.VirtualClusterEvents{}.DefaultObject())
	require.False(t, presentDiags.HasError())

	tests := []struct {
		name        string
		config      types.Object
		plan        types.Object
		wantManaged bool
		wantError   bool
	}{
		{
			name:        "null is unmanaged",
			config:      types.ObjectNull(attrTypes),
			plan:        types.ObjectUnknown(attrTypes),
			wantManaged: false,
		},
		{
			name:        "present is managed",
			config:      present,
			plan:        present,
			wantManaged: true,
		},
		{
			name:        "unknown config resolving to an object is managed",
			config:      types.ObjectUnknown(attrTypes),
			plan:        present,
			wantManaged: true,
		},
		{
			name:   "unknown config resolving to null is unmanaged",
			config: types.ObjectUnknown(attrTypes),
			plan:   types.ObjectNull(attrTypes),
		},
		{
			name:      "unknown config and plan are rejected",
			config:    types.ObjectUnknown(attrTypes),
			plan:      types.ObjectUnknown(attrTypes),
			wantError: true,
		},
		{
			name:      "present config with an unknown plan is rejected",
			config:    present,
			plan:      types.ObjectUnknown(attrTypes),
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics
			managed := eventsConfigured(test.config, test.plan, &diags)

			require.Equal(t, test.wantManaged, managed)
			require.Equal(t, test.wantError, diags.HasError(), diags.Errors())
			if test.wantError {
				require.Equal(t, "Unresolved Virtual Cluster Events Configuration", diags.Errors()[0].Summary())
			}
		})
	}
}

func TestApplyEventsSkipsUpdateWhenUnmanaged(t *testing.T) {
	t.Parallel()

	state := newEventsTestServerState()
	server := newEventsTestServer(state)
	t.Cleanup(server.Close)

	client := newEventsTestClient(t, server.URL)
	r := &virtualClusterResource{client: client}

	plan := eventsTestPlan(t, false)
	tfState := newVirtualClusterTFState(t)

	var diags diag.Diagnostics
	r.applyEvents(context.Background(), plan, &tfState, &diags, false)
	require.False(t, diags.HasError(), diags.Errors())

	state.mu.Lock()
	updateEventsCalls := state.updateEventsCalls
	getEventsCalls := state.getEventsCalls
	state.mu.Unlock()
	require.Equal(t, 0, updateEventsCalls)
	require.Equal(t, 1, getEventsCalls)

	var eventTypes types.Map
	readDiags := tfState.GetAttribute(
		context.Background(),
		path.Root("events").AtName("event_types"),
		&eventTypes,
	)
	require.False(t, readDiags.HasError(), readDiags.Errors())
	require.True(t, eventTypes.IsNull(), "unconfigured backend event types should not be tracked")
}

func TestApplyEventsWritesWhenManaged(t *testing.T) {
	t.Parallel()

	state := newEventsTestServerState()
	server := newEventsTestServer(state)
	t.Cleanup(server.Close)

	client := newEventsTestClient(t, server.URL)
	r := &virtualClusterResource{client: client}

	plan := eventsTestPlan(t, true)
	tfState := newVirtualClusterTFState(t)

	var diags diag.Diagnostics
	r.applyEvents(context.Background(), plan, &tfState, &diags, true)
	require.False(t, diags.HasError(), diags.Errors())

	state.mu.Lock()
	defer state.mu.Unlock()
	require.Equal(t, 1, state.updateEventsCalls)
	require.Equal(t, 1, state.getEventsCalls)
	require.NotNil(t, state.lastUpdateEnabled)
	require.True(t, *state.lastUpdateEnabled)
}

func TestApplyEventsRejectsUnresolvedManagedPlan(t *testing.T) {
	t.Parallel()

	state := newEventsTestServerState()
	server := newEventsTestServer(state)
	t.Cleanup(server.Close)

	client := newEventsTestClient(t, server.URL)
	r := &virtualClusterResource{client: client}

	plan := eventsTestPlan(t, true)
	plan.Events = types.ObjectUnknown(models.VirtualClusterEvents{}.AttributeTypes())
	tfState := newVirtualClusterTFState(t)

	var diags diag.Diagnostics
	r.applyEvents(context.Background(), plan, &tfState, &diags, true)
	require.True(t, diags.HasError())
	require.Equal(t, "Unresolved Virtual Cluster Events Configuration", diags.Errors()[0].Summary())

	state.mu.Lock()
	updateEventsCalls := state.updateEventsCalls
	getEventsCalls := state.getEventsCalls
	state.mu.Unlock()
	require.Equal(t, 0, updateEventsCalls)
	require.Equal(t, 0, getEventsCalls)
}

type eventsTestServerState struct {
	mu                sync.Mutex
	getEventsCalls    int
	updateEventsCalls int
	lastUpdateEnabled *bool
	enabled           bool
	eventTypes        map[string]api.EventTypeConfig
}

func newEventsTestServerState() *eventsTestServerState {
	eventTypeEnabled := true
	return &eventsTestServerState{
		enabled: true,
		eventTypes: map[string]api.EventTypeConfig{
			"backend_only": {
				Enabled: &eventTypeEnabled,
			},
		},
	}
}

func newEventsTestServer(state *eventsTestServerState) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/get_events_state":
			state.mu.Lock()
			state.getEventsCalls++
			enabled := state.enabled
			eventTypes := state.eventTypes
			state.mu.Unlock()
			_ = json.NewEncoder(w).Encode(api.EventsStateDescribeResponse{
				Enabled:    enabled,
				EventTypes: eventTypes,
			})
		case "/update_events_state":
			var req api.EventsStateUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			state.mu.Lock()
			state.updateEventsCalls++
			state.lastUpdateEnabled = req.Enabled
			if req.Enabled != nil {
				state.enabled = *req.Enabled
			}
			state.mu.Unlock()
			_, _ = w.Write([]byte("{}"))
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
}

func newEventsTestClient(t *testing.T, host string) *api.Client {
	t.Helper()
	token := "test-token"
	client, err := api.NewClient(host, &token, "test")
	require.NoError(t, err)
	return client
}

func eventsTestPlan(t *testing.T, enabled bool) models.VirtualClusterResource {
	t.Helper()

	eventsObj, diags := types.ObjectValue(
		models.VirtualClusterEvents{}.AttributeTypes(),
		map[string]attr.Value{
			"enabled": types.BoolValue(enabled),
			"event_types": types.MapNull(types.ObjectType{
				AttrTypes: models.EventTypeConfig{}.AttributeTypes(),
			}),
		},
	)
	require.False(t, diags.HasError(), diags.Errors())

	return models.VirtualClusterResource{
		ID:     types.StringValue("vci_test"),
		Name:   types.StringValue("vcn_test"),
		Type:   types.StringValue("byoc"),
		Events: eventsObj,
	}
}

func newVirtualClusterTFState(t *testing.T) tfsdk.State {
	t.Helper()

	var schemaResp resource.SchemaResponse
	(&virtualClusterResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	raw := tftypes.NewValue(schemaResp.Schema.Type().TerraformType(context.Background()), nil)
	return tfsdk.State{
		Raw:    raw,
		Schema: schemaResp.Schema,
	}
}
