package plugintest

import (
	"fmt"
	"strings"

	"github.com/eliben/watgo"
	"github.com/eliben/watgo/wasmir"
)

// ABIError describes one ABI conformance violation.
type ABIError struct {
	Export string
	Detail string
}

func (e ABIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Export, e.Detail)
}

// ABIReport is the result of ValidateABI.
type ABIReport struct {
	Errors   []ABIError
	Warnings []string
}

// Valid returns true if no errors were found.
func (r ABIReport) Valid() bool {
	return len(r.Errors) == 0
}

// Error returns a combined error message, or nil if valid.
func (r ABIReport) Error() error {
	if r.Valid() {
		return nil
	}
	msgs := make([]string, len(r.Errors))
	for i, e := range r.Errors {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("ABI validation failed:\n  %s", strings.Join(msgs, "\n  "))
}

// ValidateABI performs a thorough static analysis of a compiled .wasm binary
// to verify it conforms to the Axe plugin ABI contract.
//
// It checks:
//   - All required exports exist and are functions
//   - Export function signatures match expected types
//   - Expected axe_kernel host imports (if declared) have correct signatures
//   - The allocate export has the correct signature
func ValidateABI(wasmBytes []byte) ABIReport {
	var report ABIReport

	module, err := watgo.DecodeWASM(wasmBytes)
	if err != nil {
		report.Errors = append(report.Errors, ABIError{
			Export: "(module)",
			Detail: fmt.Sprintf("failed to decode wasm: %v", err),
		})
		return report
	}

	// Count imported functions to compute the function index offset.
	// In wasm, the function index space is: imported functions first,
	// then module-defined functions.
	numImportedFuncs := 0
	for _, imp := range module.Imports {
		if imp.Kind == wasmir.ExternalKindFunction {
			numImportedFuncs++
		}
	}

	exports := make(map[string]wasmir.Export)
	for _, exp := range module.Exports {
		exports[exp.Name] = exp
	}

	// Check required exports with signatures.
	// _initialize: () -> ()
	checkExportSig(module, exports, "_initialize", nil, nil, numImportedFuncs, &report)
	// Metadata: () -> (i64)
	checkExportSig(module, exports, "Metadata", nil, []wasmir.ValueType{wasmir.ValueTypeI64}, numImportedFuncs, &report)
	// Execute: (i32, i32) -> (i64)
	checkExportSig(module, exports, "Execute", []wasmir.ValueType{wasmir.ValueTypeI32, wasmir.ValueTypeI32}, []wasmir.ValueType{wasmir.ValueTypeI64}, numImportedFuncs, &report)
	// allocate: (i32) -> (i32)
	checkExportSig(module, exports, "allocate", []wasmir.ValueType{wasmir.ValueTypeI32}, []wasmir.ValueType{wasmir.ValueTypeI32}, numImportedFuncs, &report)

	checkHostImports(module, &report)

	return report
}

func checkExportSig(module *wasmir.Module, exports map[string]wasmir.Export,
	name string, wantParams, wantResults []wasmir.ValueType,
	numImportedFuncs int, report *ABIReport) {

	exp, ok := exports[name]
	if !ok {
		report.Errors = append(report.Errors, ABIError{
			Export: name,
			Detail: "missing required export",
		})
		return
	}

	if exp.Kind != wasmir.ExternalKindFunction {
		report.Errors = append(report.Errors, ABIError{
			Export: name,
			Detail: fmt.Sprintf("expected function export, got kind %d", exp.Kind),
		})
		return
	}

	// Resolve the function's type index.
	// Export.Index is into the combined function index space:
	// indices [0, numImportedFuncs) are imported functions,
	// indices [numImportedFuncs, ...) index into module.Funcs.
	var typeIdx uint32
	if int(exp.Index) < numImportedFuncs {
		importFuncIdx := 0
		for _, imp := range module.Imports {
			if imp.Kind == wasmir.ExternalKindFunction {
				if importFuncIdx == int(exp.Index) {
					typeIdx = imp.TypeIdx
					break
				}
				importFuncIdx++
			}
		}
	} else {
		funcIdx := int(exp.Index) - numImportedFuncs
		if funcIdx < 0 || funcIdx >= len(module.Funcs) {
			report.Errors = append(report.Errors, ABIError{
				Export: name,
				Detail: fmt.Sprintf("function index %d out of range", exp.Index),
			})
			return
		}
		typeIdx = module.Funcs[funcIdx].TypeIdx
	}

	if int(typeIdx) >= len(module.Types) {
		report.Errors = append(report.Errors, ABIError{
			Export: name,
			Detail: fmt.Sprintf("type index %d out of range", typeIdx),
		})
		return
	}

	typeDef := module.Types[typeIdx]

	if !valueTypesEqual(typeDef.Params, wantParams) {
		report.Errors = append(report.Errors, ABIError{
			Export: name,
			Detail: fmt.Sprintf("parameter mismatch: got %s, want %s",
				formatTypes(typeDef.Params), formatTypes(wantParams)),
		})
	}

	if !valueTypesEqual(typeDef.Results, wantResults) {
		report.Errors = append(report.Errors, ABIError{
			Export: name,
			Detail: fmt.Sprintf("result mismatch: got %s, want %s",
				formatTypes(typeDef.Results), formatTypes(wantResults)),
		})
	}
}

func checkHostImports(module *wasmir.Module, report *ABIReport) {
	// Expected signatures for axe_kernel functions.
	expected := map[string]struct {
		params  []wasmir.ValueType
		results []wasmir.ValueType
	}{
		"track_artifact": {
			params:  []wasmir.ValueType{wasmir.ValueTypeI32, wasmir.ValueTypeI32, wasmir.ValueTypeI64},
			results: nil,
		},
		"get_budget_used": {
			params:  nil,
			results: []wasmir.ValueType{wasmir.ValueTypeI64},
		},
		"get_budget_max": {
			params:  nil,
			results: []wasmir.ValueType{wasmir.ValueTypeI64},
		},
	}

	for _, imp := range module.Imports {
		if imp.Module != "axe_kernel" {
			continue
		}
		if imp.Kind != wasmir.ExternalKindFunction {
			report.Errors = append(report.Errors, ABIError{
				Export: "import:" + imp.Name,
				Detail: "expected function import",
			})
			continue
		}

		spec, known := expected[imp.Name]
		if !known {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("unknown axe_kernel import: %s", imp.Name))
			continue
		}

		if int(imp.TypeIdx) >= len(module.Types) {
			report.Errors = append(report.Errors, ABIError{
				Export: "import:" + imp.Name,
				Detail: fmt.Sprintf("type index %d out of range", imp.TypeIdx),
			})
			continue
		}

		typeDef := module.Types[imp.TypeIdx]
		if !valueTypesEqual(typeDef.Params, spec.params) || !valueTypesEqual(typeDef.Results, spec.results) {
			report.Errors = append(report.Errors, ABIError{
				Export: "import:" + imp.Name,
				Detail: fmt.Sprintf("signature mismatch: got (%s)->(%s), want (%s)->(%s)",
					formatTypes(typeDef.Params), formatTypes(typeDef.Results),
					formatTypes(spec.params), formatTypes(spec.results)),
			})
		}
	}
}

func valueTypesEqual(a, b []wasmir.ValueType) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func formatTypes(types []wasmir.ValueType) string {
	if len(types) == 0 {
		return "()"
	}
	parts := make([]string, len(types))
	for i, t := range types {
		parts[i] = t.String()
	}
	return "(" + strings.Join(parts, ", ") + ")"
}
