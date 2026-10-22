package resources

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/api"
	"github.com/warpstreamlabs/terraform-provider-warpstream/internal/provider/models"
)

func TestGrantModelsOrderedLike(t *testing.T) {
	t.Parallel()

	model := func(workspaceID, grantType string) models.UserRoleGrant {
		return models.UserRoleGrant{WorkspaceID: types.StringValue(workspaceID), GrantType: types.StringValue(grantType)}
	}
	apiGrant := func(workspaceID, grantType string) api.AccessGrant {
		return api.AccessGrant{WorkspaceID: workspaceID, ManagedGrantKey: grantType, ResourceID: "*"}
	}

	tests := []struct {
		name      string
		reference []models.UserRoleGrant
		apiGrants []api.AccessGrant
		want      []models.UserRoleGrant
	}{
		{
			name:      "API reorders grants",
			reference: []models.UserRoleGrant{model("wi_private", "admin"), model("wi_shared", "admin"), model("*", "read_only")},
			apiGrants: []api.AccessGrant{apiGrant("*", "read_only"), apiGrant("wi_private", "admin"), apiGrant("wi_shared", "admin")},
			want:      []models.UserRoleGrant{model("wi_private", "admin"), model("wi_shared", "admin"), model("*", "read_only")},
		},
		{
			name:      "same order",
			reference: []models.UserRoleGrant{model("wi_a", "admin"), model("-", "billing")},
			apiGrants: []api.AccessGrant{apiGrant("wi_a", "admin"), apiGrant("-", "billing")},
			want:      []models.UserRoleGrant{model("wi_a", "admin"), model("-", "billing")},
		},
		{
			name:      "grant added outside terraform is appended",
			reference: []models.UserRoleGrant{model("wi_a", "admin"), model("*", "read_only")},
			apiGrants: []api.AccessGrant{apiGrant("wi_b", "admin"), apiGrant("*", "read_only"), apiGrant("wi_a", "admin")},
			want:      []models.UserRoleGrant{model("wi_a", "admin"), model("*", "read_only"), model("wi_b", "admin")},
		},
		{
			name:      "grant removed outside terraform is dropped",
			reference: []models.UserRoleGrant{model("wi_a", "admin"), model("wi_b", "admin"), model("*", "read_only")},
			apiGrants: []api.AccessGrant{apiGrant("*", "read_only"), apiGrant("wi_a", "admin")},
			want:      []models.UserRoleGrant{model("wi_a", "admin"), model("*", "read_only")},
		},
		{
			name:      "grant type changed outside terraform",
			reference: []models.UserRoleGrant{model("wi_a", "admin"), model("*", "read_only")},
			apiGrants: []api.AccessGrant{apiGrant("*", "read_only"), apiGrant("wi_a", "read_only")},
			want:      []models.UserRoleGrant{model("*", "read_only"), model("wi_a", "read_only")},
		},
		{
			name:      "duplicate grants are matched one to one",
			reference: []models.UserRoleGrant{model("wi_a", "admin"), model("*", "read_only"), model("wi_a", "admin")},
			apiGrants: []api.AccessGrant{apiGrant("wi_a", "admin"), apiGrant("*", "read_only")},
			want:      []models.UserRoleGrant{model("wi_a", "admin"), model("*", "read_only")},
		},
		{
			name:      "empty reference keeps API order",
			reference: nil,
			apiGrants: []api.AccessGrant{apiGrant("*", "read_only"), apiGrant("wi_a", "admin")},
			want:      []models.UserRoleGrant{model("*", "read_only"), model("wi_a", "admin")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, grantModelsOrderedLike(tt.reference, tt.apiGrants))
		})
	}
}
