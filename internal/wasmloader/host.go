package wasmloader

import (
	"context"
	"fmt"
	"math"

	"github.com/jrswab/axe/internal/artifact"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// HostCapabilities is the set of host functions available to plugins,
// keyed by module name then function name.
var HostCapabilities = map[string]map[string]bool{
	"axe_kernel": {
		"track_artifact":  true,
		"get_budget_used": true,
		"get_budget_max":  true,
		"play_chime":      true,
	},
}

// InstantiateHostModule registers the axe_kernel host functions in the wazero runtime.
func InstantiateHostModule(ctx context.Context, r wazero.Runtime) error {
	_, err := r.NewHostModuleBuilder("axe_kernel").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			state, ok := GetKernelState(ctx)
			if !ok || state.ArtifactTracker == nil {
				return
			}

			ptr := uint32(stack[0])
			size := uint32(stack[1])
			artifactSize := int64(stack[2])

			pathBytes, ok := m.Memory().Read(ptr, size)
			if !ok {
				return
			}

			state.ArtifactTracker.Record(artifact.Entry{
				Path:  string(pathBytes),
				Agent: state.AgentName,
				Size:  artifactSize,
			})
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI64}, []api.ValueType{}).
		Export("track_artifact").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			state, ok := GetKernelState(ctx)
			if !ok || state.BudgetTracker == nil {
				stack[0] = 0
				return
			}
			stack[0] = uint64(state.BudgetTracker.Used())
		}), []api.ValueType{}, []api.ValueType{api.ValueTypeI64}).
		Export("get_budget_used").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			state, ok := GetKernelState(ctx)
			if !ok || state.BudgetTracker == nil {
				stack[0] = 0
				return
			}
			stack[0] = uint64(state.BudgetTracker.Max())
		}), []api.ValueType{}, []api.ValueType{api.ValueTypeI64}).
		Export("get_budget_max").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			freq := math.Float64frombits(stack[0])
			decay := math.Float64frombits(stack[1])
			volume := math.Float64frombits(stack[2])
			richness := math.Float64frombits(stack[3])
			harmonicMult := math.Float64frombits(stack[4])
			playChime(freq, decay, volume, richness, harmonicMult)
		}), []api.ValueType{api.ValueTypeF64, api.ValueTypeF64, api.ValueTypeF64, api.ValueTypeF64, api.ValueTypeF64}, []api.ValueType{}).
		Export("play_chime").
		Instantiate(ctx)

	if err != nil {
		return fmt.Errorf("failed to instantiate axe_kernel host module: %w", err)
	}
	return nil
}
