package wasmloader

import (
	"context"
	"fmt"

	"github.com/jrswab/axe/internal/artifact"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

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
		Instantiate(ctx)

	if err != nil {
		return fmt.Errorf("failed to instantiate axe_kernel host module: %w", err)
	}
	return nil
}
