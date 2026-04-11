package wasmloader

import (
	"context"

	"github.com/jrswab/axe/internal/artifact"
	"github.com/jrswab/axe/internal/budget"
)

type contextKey struct{}

var kernelKey = contextKey{}

// KernelState holds the host-side state for a Wasm plugin run.
type KernelState struct {
	ArtifactTracker *artifact.Tracker
	BudgetTracker   *budget.BudgetTracker
	AgentName       string
}

// WithKernelState injects the kernel state into the context.
func WithKernelState(ctx context.Context, state KernelState) context.Context {
	return context.WithValue(ctx, kernelKey, state)
}

// GetKernelState retrieves the kernel state from the context.
func GetKernelState(ctx context.Context) (KernelState, bool) {
	state, ok := ctx.Value(kernelKey).(KernelState)
	return state, ok
}
