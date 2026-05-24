//go:build esim_transfer

package features

import (
	"slices"
	"testing"
)

func TestListESIMTransferBuildIncludesTransfer(t *testing.T) {
	t.Parallel()

	if got := List(); !slices.Contains(got, EsimTransfer) {
		t.Fatalf("List() = %v, want %q", got, EsimTransfer)
	}
}
