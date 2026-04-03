package validate

import (
	"fmt"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"go.yaml.in/yaml/v3"
)

type Field struct {
	Required  bool
	NodeKind  yaml.Kind
	Children  map[string]Field
	ItemKind  yaml.Kind
	ValueKind yaml.Kind
}

type Decls struct {
	Variables   map[string]struct{}
	Parameters  map[string]struct{}
	Observables map[string]struct{}
	Shocks      map[string]struct{}
}

var allowedTopLevel = map[string]struct{}{
	"name":        {},
	"variables":   {},
	"constrained": {},
	"parameters":  {},
	"shock_map":   {},
	"observables": {},
	"equations":   {},
	"calibration": {},
	"kalman":      {},
}

var schema = map[string]Field{
	"name": {
		Required: true,
		NodeKind: yaml.ScalarNode,
	},

	"variables": {
		Required: true,
		NodeKind: yaml.SequenceNode,
		ItemKind: yaml.ScalarNode,
	},
	"constrained": {
		Required:  false,
		NodeKind:  yaml.MappingNode,
		ValueKind: yaml.ScalarNode,
	},
	"parameters": {
		Required: true,
		NodeKind: yaml.SequenceNode,
		ItemKind: yaml.ScalarNode,
	},
	"shock_map": {
		Required:  true,
		NodeKind:  yaml.MappingNode,
		ValueKind: yaml.ScalarNode,
	},
	"observables": {
		Required: true,
		NodeKind: yaml.SequenceNode,
		ItemKind: yaml.ScalarNode,
	},
	"equations": {
		Required: true,
		NodeKind: yaml.MappingNode,
		Children: map[string]Field{
			"model": {
				Required: true,
				NodeKind: yaml.SequenceNode,
				ItemKind: yaml.ScalarNode,
			},
			"constraint": {
				Required: false,
				NodeKind: yaml.MappingNode,
			},
			"observables": {
				Required:  true,
				NodeKind:  yaml.MappingNode,
				ValueKind: yaml.ScalarNode,
			},
		},
	},
	"calibration": {
		Required: true,
		NodeKind: yaml.MappingNode,
		Children: map[string]Field{
			"parameters": {
				Required:  true,
				NodeKind:  yaml.MappingNode,
				ValueKind: yaml.ScalarNode,
			},
			"shocks": {
				Required: true,
				NodeKind: yaml.MappingNode,
				Children: map[string]Field{
					"std": {
						Required:  true,
						NodeKind:  yaml.MappingNode,
						ValueKind: yaml.ScalarNode,
					},
					"corr": {
						Required:  false,
						NodeKind:  yaml.MappingNode,
						ValueKind: yaml.ScalarNode,
					},
				},
			},
		},
	},
	"kalman": {
		Required: false,
		NodeKind: yaml.MappingNode,
		Children: map[string]Field{
			"y": {
				Required: false,
				NodeKind: yaml.SequenceNode,
				ItemKind: yaml.ScalarNode,
			},
			"R": {
				Required: true,
				NodeKind: yaml.MappingNode,
				Children: map[string]Field{
					"std": {
						Required:  true,
						NodeKind:  yaml.MappingNode,
						ValueKind: yaml.ScalarNode,
					},
					"corr": {
						Required:  false,
						NodeKind:  yaml.MappingNode,
						ValueKind: yaml.ScalarNode,
					},
				},
			},
			"P0": {
				Required: false,
				NodeKind: yaml.MappingNode,
				Children: map[string]Field{
					"mode": {
						Required: true,
						NodeKind: yaml.ScalarNode,
					},
					"scale": {
						Required: true,
						NodeKind: yaml.ScalarNode,
					},
					"diag": {
						Required:  true,
						NodeKind:  yaml.MappingNode,
						ValueKind: yaml.ScalarNode,
					},
				},
			},
			"jitter": {
				Required: false,
				NodeKind: yaml.ScalarNode,
			},
			"symmetrize": {
				Required: false,
				NodeKind: yaml.ScalarNode,
			},
		},
	},
}

func validateTopLevel(node *yaml.Node) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	if node.Kind != yaml.MappingNode {
		diags = append(diags, diagAtNode(node, protocol.DiagnosticSeverityError, "root document must be a mapping"))
		return diags
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		key := keyNode.Value

		if _, ok := allowedTopLevel[key]; !ok {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("unknown top-level field %q", key),
			))
		}
	}

	return diags
}
func Run(root *yaml.Node) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	doc := unwrapDocument(root)
	if doc == nil {
		return diags
	}

	if doc.Kind != yaml.MappingNode {
		return []protocol.Diagnostic{
			diagAtNode(doc, protocol.DiagnosticSeverityError, "root document must be a mapping"),
		}
	}

	diags = append(diags, validateMappingAgainstSchema(doc, schema, "root")...)

	decls := collectDecls(doc)
	diags = append(diags, validateDuplicateKeys(doc)...)
	diags = append(diags, validateDeclarationSequences(doc)...)
	diags = append(diags, validateCrossRefs(doc, decls)...)

	return diags
}

func validateCrossRefs(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	diags = append(diags, validateConstrained(doc, decls)...)
	diags = append(diags, validateShockMap(doc, decls)...)
	diags = append(diags, validateCalibrationParameters(doc, decls)...)
	diags = append(diags, validateCalibrationShockKeys(doc, decls)...)
	diags = append(diags, validateCalibrationShockValues(doc, decls)...)
	diags = append(diags, validateEquationObservableKeys(doc, decls)...)
	diags = append(diags, validateKalmanRStd(doc, decls)...)
	diags = append(diags, validateKalmanRCorr(doc, decls)...)
	diags = append(diags, validateKalmanY(doc, decls)...)
	diags = append(diags, validateKalmanP0Diag(doc, decls)...)
	diags = append(diags, validateScalarRules(doc)...)
	diags = append(diags, validateEquations(doc, decls)...)

	return diags
}

func getMapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		if keyNode.Value == key {
			return valNode
		}
	}

	return nil
}

func sequenceToSet(node *yaml.Node) map[string]struct{} {
	out := map[string]struct{}{}

	if node == nil || node.Kind != yaml.SequenceNode {
		return out
	}

	for _, item := range node.Content {
		if item.Kind == yaml.ScalarNode {
			out[item.Value] = struct{}{}
		}
	}

	return out
}

func mappingKeysToSet(node *yaml.Node) map[string]struct{} {
	out := map[string]struct{}{}

	if node == nil || node.Kind != yaml.MappingNode {
		return out
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if keyNode.Kind == yaml.ScalarNode {
			out[keyNode.Value] = struct{}{}
		}
	}

	return out
}

func has(set map[string]struct{}, name string) bool {
	_, ok := set[name]
	return ok
}

func validateConstrained(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	node := getMapValue(doc, "constrained")
	if node == nil || node.Kind != yaml.MappingNode {
		return diags
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		name := keyNode.Value

		if !has(decls.Variables, name) {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("key %q in constrained is not declared in variables", name),
			))
		}
	}

	return diags
}

func validateShockMap(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	node := getMapValue(doc, "shock_map")
	if node == nil || node.Kind != yaml.MappingNode {
		return diags
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		if valNode.Kind != yaml.ScalarNode {
			continue
		}

		if !has(decls.Variables, valNode.Value) {
			diags = append(diags, diagAtNode(
				valNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("value %q in shock_map.%s is not declared in variables", valNode.Value, keyNode.Value),
			))
		}
	}

	return diags
}

func validateCalibrationParameters(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	calibration := getMapValue(doc, "calibration")
	if calibration == nil || calibration.Kind != yaml.MappingNode {
		return diags
	}

	paramsNode := getMapValue(calibration, "parameters")
	if paramsNode == nil || paramsNode.Kind != yaml.MappingNode {
		return diags
	}

	seen := map[string]struct{}{}

	for i := 0; i < len(paramsNode.Content); i += 2 {
		keyNode := paramsNode.Content[i]
		name := keyNode.Value
		seen[name] = struct{}{}

		if !has(decls.Parameters, name) {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("key %q in calibration.parameters is not declared in parameters", name),
			))
		}
	}

	for name := range decls.Parameters {
		if !has(seen, name) {
			diags = append(diags, diagAtNode(
				paramsNode,
				protocol.DiagnosticSeverityWarning,
				fmt.Sprintf("declared parameter %q is missing from calibration.parameters", name),
			))
		}
	}

	return diags
}

func validateCalibrationShockValues(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	calibration := getMapValue(doc, "calibration")
	if calibration == nil || calibration.Kind != yaml.MappingNode {
		return diags
	}

	shocks := getMapValue(calibration, "shocks")
	if shocks == nil || shocks.Kind != yaml.MappingNode {
		return diags
	}

	stdNode := getMapValue(shocks, "std")
	diags = append(diags, validateMappingValuesInSet(stdNode, decls.Parameters, "calibration.shocks.std", "parameters")...)

	corrNode := getMapValue(shocks, "corr")
	diags = append(diags, validateMappingValuesInSet(corrNode, decls.Parameters, "calibration.shocks.corr", "parameters")...)

	return diags
}

func validateKalmanY(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	kalman := getMapValue(doc, "kalman")
	if kalman == nil || kalman.Kind != yaml.MappingNode {
		return diags
	}

	yNode := getMapValue(kalman, "y")
	if yNode == nil || yNode.Kind != yaml.SequenceNode {
		return diags
	}

	for _, item := range yNode.Content {
		if item.Kind != yaml.ScalarNode {
			continue
		}

		if !has(decls.Observables, item.Value) {
			diags = append(diags, diagAtNode(
				item,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("value %q in kalman.y is not declared in observables", item.Value),
			))
		}
	}

	return diags
}

func validateKalmanP0Diag(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	kalman := getMapValue(doc, "kalman")
	if kalman == nil || kalman.Kind != yaml.MappingNode {
		return diags
	}

	p0 := getMapValue(kalman, "P0")
	if p0 == nil || p0.Kind != yaml.MappingNode {
		return diags
	}

	diagNode := getMapValue(p0, "diag")
	if diagNode == nil || diagNode.Kind != yaml.MappingNode {
		return diags
	}

	for i := 0; i < len(diagNode.Content); i += 2 {
		keyNode := diagNode.Content[i]
		name := keyNode.Value

		if !has(decls.Variables, name) {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("key %q in kalman.P0.diag is not declared in variables", name),
			))
		}
	}

	return diags
}

func validateMappingValuesInSet(node *yaml.Node, allowed map[string]struct{}, path string, declName string) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	if node == nil || node.Kind != yaml.MappingNode {
		return diags
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]

		if valNode.Kind != yaml.ScalarNode {
			continue
		}

		if !has(allowed, valNode.Value) {
			diags = append(diags, diagAtNode(
				valNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("value %q in %s.%s is not declared in %s", valNode.Value, path, keyNode.Value, declName),
			))
		}
	}

	return diags
}

func collectDecls(doc *yaml.Node) Decls {
	return Decls{
		Variables:   sequenceToSet(getMapValue(doc, "variables")),
		Parameters:  sequenceToSet(getMapValue(doc, "parameters")),
		Observables: sequenceToSet(getMapValue(doc, "observables")),
		Shocks:      mappingKeysToSet(getMapValue(doc, "shock_map")),
	}
}

func unwrapDocument(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		return root.Content[0]
	}
	return root
}

func validateNode(node *yaml.Node, field Field, path string) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	if node == nil {
		return diags
	}

	if node.Kind != field.NodeKind {
		diags = append(diags, diagAtNode(
			node,
			protocol.DiagnosticSeverityError,
			fmt.Sprintf("%s must be %s", path, kindName(field.NodeKind)),
		))
		return diags
	}

	switch node.Kind {
	case yaml.SequenceNode:
		diags = append(diags, validateSequence(node, field, path)...)
	case yaml.MappingNode:
		diags = append(diags, validateMapping(node, field, path)...)
	}

	return diags
}

func validateSequence(node *yaml.Node, field Field, path string) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	if field.ItemKind == 0 {
		return diags
	}

	for i, item := range node.Content {
		if item.Kind != field.ItemKind {
			diags = append(diags, diagAtNode(
				item,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("%s[%d] must be %s", path, i, kindName(field.ItemKind)),
			))
		}
	}

	return diags
}

func validateMapping(node *yaml.Node, field Field, path string) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	// Case 1: fixed child schema
	if field.Children != nil {
		diags = append(diags, validateMappingAgainstSchema(node, field.Children, path)...)
		return diags
	}

	// Case 2: free-form mapping with constrained value kind
	if field.ValueKind != 0 {
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			if valNode.Kind != field.ValueKind {
				diags = append(diags, diagAtNode(
					valNode,
					protocol.DiagnosticSeverityError,
					fmt.Sprintf("%s.%s must be %s", path, keyNode.Value, kindName(field.ValueKind)),
				))
			}
		}
	}

	return diags
}

func validateMappingAgainstSchema(node *yaml.Node, spec map[string]Field, path string) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	if node.Kind != yaml.MappingNode {
		diags = append(diags, diagAtNode(
			node,
			protocol.DiagnosticSeverityError,
			fmt.Sprintf("%s must be a mapping", path),
		))
		return diags
	}

	seen := map[string]*yaml.Node{}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		key := keyNode.Value

		field, ok := spec[key]
		if !ok {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("unknown field %q in %s", key, path),
			))
			continue
		}

		seen[key] = valNode
		childPath := joinPath(path, key)
		diags = append(diags, validateNode(valNode, field, childPath)...)
	}

	for key, field := range spec {
		if field.Required {
			if _, ok := seen[key]; !ok {
				diags = append(diags, diagAtNode(
					node,
					protocol.DiagnosticSeverityError,
					fmt.Sprintf("missing required field %q in %s", key, path),
				))
			}
		}
	}

	return diags
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	if parent == "root" {
		return child
	}
	return parent + "." + child
}

func kindName(kind yaml.Kind) string {
	switch kind {
	case yaml.ScalarNode:
		return "a scalar"
	case yaml.SequenceNode:
		return "a sequence"
	case yaml.MappingNode:
		return "a mapping"
	default:
		return "the expected YAML kind"
	}
}

func diagAtNode(node *yaml.Node, sev protocol.DiagnosticSeverity, msg string) protocol.Diagnostic {
	source := "sdsge-ls"

	line := 0
	col := 0
	endCol := 1

	if node != nil {
		if node.Line > 0 {
			line = node.Line - 1
		}
		if node.Column > 0 {
			col = node.Column - 1
			endCol = col + max(1, len(node.Value))
		}
	}

	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(line), Character: uint32(col)},
			End:   protocol.Position{Line: uint32(line), Character: uint32(endCol)},
		},
		Severity: &sev,
		Source:   &source,
		Message:  msg,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
