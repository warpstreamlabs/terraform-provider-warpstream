package models

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSecretOrPrior(t *testing.T) {
	prior := types.StringValue("aks_v2_from_state")

	if got := SecretOrPrior("aks_listed", prior); got.ValueString() != "aks_listed" {
		t.Fatalf("listed secret should win, got %q", got.ValueString())
	}
	if got := SecretOrPrior("", prior); !got.Equal(prior) {
		t.Fatalf("empty listed secret should keep state, got %q", got.ValueString())
	}
}
