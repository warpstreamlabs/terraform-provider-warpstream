package resources

import (
	"context"

	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
)

// ownedKeyCapture asks a v1 resource's Create to create its object through the v2 endpoint, and receives
// the object ID and the key it came with, whose secret is only in the create response.
type ownedKeyCapture struct {
	objectID string
	key      *api.APIKey
}

type ownedKeyCaptureCtxKey struct{}

func withOwnedKeyCapture(ctx context.Context) (context.Context, *ownedKeyCapture) {
	capture := &ownedKeyCapture{}
	return context.WithValue(ctx, ownedKeyCaptureCtxKey{}, capture), capture
}

// ownedKeyCaptureFrom returns nil unless the call comes from a v2 resource.
func ownedKeyCaptureFrom(ctx context.Context) *ownedKeyCapture {
	capture, _ := ctx.Value(ownedKeyCaptureCtxKey{}).(*ownedKeyCapture)
	return capture
}

func (c *ownedKeyCapture) record(objectID string, key *api.APIKey) {
	if c == nil {
		return
	}
	c.objectID = objectID
	c.key = key
}

func (c *ownedKeyCapture) recordCluster(cluster *api.VirtualCluster) {
	if c == nil || cluster.AgentKeys == nil || len(*cluster.AgentKeys) == 0 {
		return
	}
	key := (*cluster.AgentKeys)[0]
	c.record(cluster.ID, &key)
}
