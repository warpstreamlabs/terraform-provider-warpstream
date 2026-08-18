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

	require.False(t, eventsConfigured(types.ObjectNull(attrTypes)))
	require.False(t, eventsConfigured(types.ObjectUnknown(attrTypes)))

	present, diags := types.ObjectValue(attrTypes, models.VirtualClusterEvents{}.DefaultObject())
	require.False(t, diags.HasError())
	require.True(t, eventsConfigured(present))
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
	defer state.mu.Unlock()
	require.Equal(t, 0, state.updateEventsCalls)
	require.Equal(t, 1, state.getEventsCalls)
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

type eventsTestServerState struct {
	mu                sync.Mutex
	getEventsCalls    int
	updateEventsCalls int
	lastUpdateEnabled *bool
	enabled           bool
}

func newEventsTestServerState() *eventsTestServerState {
	return &eventsTestServerState{enabled: true}
}

func newEventsTestServer(state *eventsTestServerState) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/get_events_state":
			state.mu.Lock()
			state.getEventsCalls++
			enabled := state.enabled
			state.mu.Unlock()
			_ = json.NewEncoder(w).Encode(api.EventsStateDescribeResponse{
				Enabled:    enabled,
				EventTypes: map[string]api.EventTypeConfig{},
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
