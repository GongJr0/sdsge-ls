package validate

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/GongJr0/sdsge-ls/internal/expr"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"go.yaml.in/yaml/v3"
)

var (
	modelLHSRE                  = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*t(?:\s*[+-]\s*\d+)?\s*\)\s*$`)
	allowedLinearizationMethods = map[string]struct{}{
		"log":    {},
		"none":   {},
		"taylor": {},
	}
	builtinFns = map[string]struct{}{
		"abs":  {},
		"cos":  {},
		"exp":  {},
		"log":  {},
		"max":  {},
		"min":  {},
		"pow":  {},
		"sin":  {},
		"sqrt": {},
		"tan":  {},
	}
	builtinNames = map[string]struct{}{
		"pi":    {},
		"true":  {},
		"false": {},
		"t":     {},
	}
)

func getNestedMapValue(node *yaml.Node, keys ...string) *yaml.Node {
	current := node
	for _, key := range keys {
		current = getMapValue(current, key)
		if current == nil {
			return nil
		}
	}
	return current
}

func validateDuplicateKeys(node *yaml.Node) []protocol.Diagnostic {
	var diags []protocol.Diagnostic
	if node == nil {
		return diags
	}

	if node.Kind == yaml.MappingNode {
		seen := map[string]*yaml.Node{}
		for i := 0; i < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			if prev, ok := seen[keyNode.Value]; ok {
				_ = prev
				diags = append(diags, diagAtNode(
					keyNode,
					protocol.DiagnosticSeverityError,
					fmt.Sprintf("duplicate key %q", keyNode.Value),
				))
			} else {
				seen[keyNode.Value] = keyNode
			}

			diags = append(diags, validateDuplicateKeys(valNode)...)
		}
		return diags
	}

	for _, child := range node.Content {
		diags = append(diags, validateDuplicateKeys(child)...)
	}

	return diags
}

func validateDeclarationSequences(doc *yaml.Node) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	diags = append(diags, validateVariableDeclarations(getMapValue(doc, "variables"))...)
	diags = append(diags, validateDeclarationSequence(getMapValue(doc, "parameters"), "parameters")...)
	diags = append(diags, validateDeclarationSequence(getMapValue(doc, "observables"), "observables")...)

	return diags
}

func validateVariableDeclarations(node *yaml.Node) []protocol.Diagnostic {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	return validateDeclarationSequence(node, "variables")
}

func validateDeclarationSequence(node *yaml.Node, name string) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	if node == nil || node.Kind != yaml.SequenceNode {
		return diags
	}

	seen := map[string]*yaml.Node{}
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			continue
		}

		if _, ok := seen[item.Value]; ok {
			diags = append(diags, diagAtNode(
				item,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("duplicate declaration %q in %s", item.Value, name),
			))
			continue
		}

		seen[item.Value] = item
	}

	return diags
}

func validateCalibrationShockKeys(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	stdNode := getNestedMapValue(doc, "calibration", "shocks", "std")
	if stdNode != nil && stdNode.Kind == yaml.MappingNode {
		seen := map[string]struct{}{}
		for i := 0; i < len(stdNode.Content); i += 2 {
			keyNode := stdNode.Content[i]
			name := keyNode.Value
			seen[name] = struct{}{}

			if !has(decls.Shocks, name) {
				diags = append(diags, diagAtNode(
					keyNode,
					protocol.DiagnosticSeverityError,
					fmt.Sprintf("key %q in calibration.shocks.std is not declared in shock_map", name),
				))
			}
		}

		for _, name := range sortedNames(decls.Shocks) {
			if !has(seen, name) {
				diags = append(diags, diagAtNode(
					stdNode,
					protocol.DiagnosticSeverityWarning,
					fmt.Sprintf("declared shock %q is missing from calibration.shocks.std", name),
				))
			}
		}
	}

	corrNode := getNestedMapValue(doc, "calibration", "shocks", "corr")
	diags = append(diags, validatePairMappingKeys(corrNode, decls.Shocks, "calibration.shocks.corr", "shock_map")...)

	return diags
}

func validateEquationObservableKeys(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	node := getNestedMapValue(doc, "equations", "observables")
	if node == nil || node.Kind != yaml.MappingNode {
		return diags
	}

	seen := map[string]struct{}{}
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		name := keyNode.Value
		seen[name] = struct{}{}

		if !has(decls.Observables, name) {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("key %q in equations.observables is not declared in observables", name),
			))
		}
	}

	for _, name := range sortedNames(decls.Observables) {
		if !has(seen, name) {
			diags = append(diags, diagAtNode(
				node,
				protocol.DiagnosticSeverityWarning,
				fmt.Sprintf("declared observable %q is missing from equations.observables", name),
			))
		}
	}

	return diags
}

func validateKalmanRStd(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	node := getNestedMapValue(doc, "kalman", "R", "std")
	if node == nil || node.Kind != yaml.MappingNode {
		return diags
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if !has(decls.Observables, keyNode.Value) {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("key %q in kalman.R.std is not declared in observables", keyNode.Value),
			))
		}
	}

	diags = append(diags, validateMappingValuesInSet(node, decls.Parameters, "kalman.R.std", "parameters")...)
	return diags
}

func validateKalmanRCorr(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	node := getNestedMapValue(doc, "kalman", "R", "corr")

	diags := validatePairMappingKeys(node, decls.Observables, "kalman.R.corr", "observables")
	diags = append(diags, validateMappingValuesInSet(node, decls.Parameters, "kalman.R.corr", "parameters")...)
	return diags
}

func validateScalarRules(doc *yaml.Node) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	variables := getMapValue(doc, "variables")
	if variables != nil && variables.Kind == yaml.MappingNode {
		for i := 0; i < len(variables.Content); i += 2 {
			nameNode := variables.Content[i]
			specNode := variables.Content[i+1]
			if specNode == nil || specNode.Kind != yaml.MappingNode {
				continue
			}

			linearizationNode := getMapValue(specNode, "linearization")
			if linearizationNode != nil && linearizationNode.Kind == yaml.ScalarNode && !isNullNode(linearizationNode) {
				method := strings.ToLower(strings.TrimSpace(linearizationNode.Value))
				if _, ok := allowedLinearizationMethods[method]; !ok {
					diags = append(diags, diagAtNode(
						linearizationNode,
						protocol.DiagnosticSeverityError,
						fmt.Sprintf(
							"variables.%s.linearization must be one of: log, none, taylor",
							nameNode.Value,
						),
					))
				}
			}
		}
	}

	constrained := getMapValue(doc, "constrained")
	if constrained != nil && constrained.Kind == yaml.MappingNode {
		for i := 1; i < len(constrained.Content); i += 2 {
			valNode := constrained.Content[i]
			if !isBoolScalar(valNode) {
				diags = append(diags, diagAtNode(
					valNode,
					protocol.DiagnosticSeverityError,
					"constrained values must be booleans",
				))
			}
		}
	}

	calibrationParams := getNestedMapValue(doc, "calibration", "parameters")
	if calibrationParams != nil && calibrationParams.Kind == yaml.MappingNode {
		for i := 1; i < len(calibrationParams.Content); i += 2 {
			valNode := calibrationParams.Content[i]
			if !isNumberScalar(valNode) {
				keyNode := calibrationParams.Content[i-1]
				diags = append(diags, diagAtNode(
					valNode,
					protocol.DiagnosticSeverityError,
					fmt.Sprintf("value for calibration.parameters.%s must be numeric", keyNode.Value),
				))
			}
		}
	}

	p0 := getNestedMapValue(doc, "kalman", "P0")
	if p0 != nil && p0.Kind == yaml.MappingNode {
		modeNode := getMapValue(p0, "mode")
		if modeNode != nil && modeNode.Kind == yaml.ScalarNode {
			if modeNode.Value != "diag" && modeNode.Value != "eye" {
				diags = append(diags, diagAtNode(
					modeNode,
					protocol.DiagnosticSeverityError,
					`kalman.P0.mode must be "diag" or "eye"`,
				))
			}
		}

		scaleNode := getMapValue(p0, "scale")
		if scaleNode != nil && !isNumberScalar(scaleNode) {
			diags = append(diags, diagAtNode(
				scaleNode,
				protocol.DiagnosticSeverityError,
				"kalman.P0.scale must be numeric",
			))
		}

		diagNode := getMapValue(p0, "diag")
		if diagNode != nil && diagNode.Kind == yaml.MappingNode {
			for i := 1; i < len(diagNode.Content); i += 2 {
				valNode := diagNode.Content[i]
				if !isNumberScalar(valNode) {
					keyNode := diagNode.Content[i-1]
					diags = append(diags, diagAtNode(
						valNode,
						protocol.DiagnosticSeverityError,
						fmt.Sprintf("value for kalman.P0.diag.%s must be numeric", keyNode.Value),
					))
				}
			}
		}
	}

	jitterNode := getNestedMapValue(doc, "kalman", "jitter")
	if jitterNode != nil && !isNumberScalar(jitterNode) {
		diags = append(diags, diagAtNode(
			jitterNode,
			protocol.DiagnosticSeverityError,
			"kalman.jitter must be numeric",
		))
	}

	symmetrizeNode := getNestedMapValue(doc, "kalman", "symmetrize")
	if symmetrizeNode != nil && !isBoolScalar(symmetrizeNode) {
		diags = append(diags, diagAtNode(
			symmetrizeNode,
			protocol.DiagnosticSeverityError,
			"kalman.symmetrize must be boolean",
		))
	}

	return diags
}

func validateVariableMetadataExpressions(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	variables := getMapValue(doc, "variables")
	if variables == nil || variables.Kind != yaml.MappingNode {
		return diags
	}

	for i := 0; i < len(variables.Content); i += 2 {
		nameNode := variables.Content[i]
		specNode := variables.Content[i+1]
		if specNode == nil || specNode.Kind != yaml.MappingNode {
			continue
		}

		steadyStateNode := getMapValue(specNode, "steady_state")
		if steadyStateNode == nil || steadyStateNode.Kind != yaml.ScalarNode || isNullNode(steadyStateNode) {
			continue
		}

		context := fmt.Sprintf("variables.%s.steady_state", nameNode.Value)
		diags = append(
			diags,
			validateExpressionRefs(steadyStateNode, steadyStateNode.Value, 0, decls, context)...,
		)
	}

	return diags
}

func validateEquations(doc *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	modelNode := getNestedMapValue(doc, "equations", "model")
	if modelNode != nil && modelNode.Kind == yaml.SequenceNode {
		for _, item := range modelNode.Content {
			if item.Kind != yaml.ScalarNode {
				continue
			}
			diags = append(diags, validateModelEquation(item, decls)...)
		}
	}

	obsNode := getNestedMapValue(doc, "equations", "observables")
	if obsNode != nil && obsNode.Kind == yaml.MappingNode {
		for i := 0; i < len(obsNode.Content); i += 2 {
			keyNode := obsNode.Content[i]
			valNode := obsNode.Content[i+1]
			if valNode.Kind != yaml.ScalarNode {
				continue
			}

			context := fmt.Sprintf("equations.observables.%s", keyNode.Value)
			diags = append(diags, validateExpressionRefs(valNode, valNode.Value, 0, decls, context)...)
		}
	}

	return diags
}

func validateModelEquation(node *yaml.Node, decls Decls) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	eq := node.Value
	equals := strings.Index(eq, "=")
	if equals < 0 {
		diags = append(diags, diagAtNode(
			node,
			protocol.DiagnosticSeverityError,
			`equations.model entries must contain "="`,
		))
		return diags
	}

	lhs := strings.TrimSpace(eq[:equals])
	match := modelLHSRE.FindStringSubmatchIndex(lhs)
	if match == nil {
		diags = append(diags, diagAtNode(
			node,
			protocol.DiagnosticSeverityError,
			"left-hand side of model equation must be a time-indexed declared variable",
		))
	} else {
		name := lhs[match[2]:match[3]]
		if !has(decls.Variables, name) {
			diags = append(diags, diagAtScalarOffset(
				node,
				strings.Index(eq[:equals], name),
				strings.Index(eq[:equals], name)+len(name),
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("left-hand side variable %q is not declared in variables", name),
			))
		}
	}

	rhs := eq[equals+1:]
	diags = append(diags, validateExpressionRefs(node, rhs, equals+1, decls, "equations.model")...)

	return diags
}

func validateExpressionRefs(node *yaml.Node, expression string, baseOffset int, decls Decls, context string) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	for _, ident := range expr.FindIdentifiers(expression) {
		name := ident.Name

		if _, ok := builtinNames[strings.ToLower(name)]; ok {
			continue
		}
		if ident.Function {
			if _, ok := builtinFns[strings.ToLower(name)]; ok {
				continue
			}
		}

		if ident.TimeIndexed {
			if !has(decls.Variables, name) {
				diags = append(diags, diagAtScalarOffset(
					node,
					baseOffset+ident.Start,
					baseOffset+ident.End,
					protocol.DiagnosticSeverityError,
					fmt.Sprintf("variable reference %q in %s is not declared in variables", name, context),
				))
			}
			continue
		}

		if has(decls.Parameters, name) || has(decls.Shocks, name) || has(decls.Variables, name) {
			continue
		}

		diags = append(diags, diagAtScalarOffset(
			node,
			baseOffset+ident.Start,
			baseOffset+ident.End,
			protocol.DiagnosticSeverityError,
			fmt.Sprintf("unknown identifier %q in %s", name, context),
		))
	}

	return diags
}

func validatePairMappingKeys(node *yaml.Node, allowed map[string]struct{}, path string, declName string) []protocol.Diagnostic {
	var diags []protocol.Diagnostic

	if node == nil || node.Kind != yaml.MappingNode {
		return diags
	}

	seen := map[string]struct{}{}
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		parts := splitPairKey(keyNode.Value)
		if len(parts) != 2 {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("key %q in %s must contain exactly two comma-separated names", keyNode.Value, path),
			))
			continue
		}

		if parts[0] == parts[1] {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("key %q in %s must refer to two distinct names", keyNode.Value, path),
			))
		}

		for _, name := range parts {
			if !has(allowed, name) {
				diags = append(diags, diagAtNode(
					keyNode,
					protocol.DiagnosticSeverityError,
					fmt.Sprintf("name %q in %s is not declared in %s", name, path, declName),
				))
			}
		}

		normalized := normalizePair(parts[0], parts[1])
		if _, ok := seen[normalized]; ok {
			diags = append(diags, diagAtNode(
				keyNode,
				protocol.DiagnosticSeverityError,
				fmt.Sprintf("pair %q in %s is duplicated", keyNode.Value, path),
			))
			continue
		}

		seen[normalized] = struct{}{}
	}

	return diags
}

func splitPairKey(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}

	return out
}

func normalizePair(a, b string) string {
	parts := []string{strings.TrimSpace(a), strings.TrimSpace(b)}
	sort.Strings(parts)
	return parts[0] + "," + parts[1]
}

func isBoolScalar(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.ScalarNode {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(node.Value)) {
	case "true", "false":
		return true
	default:
		return node.Tag == "!!bool"
	}
}

func isNumberScalar(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.ScalarNode {
		return false
	}

	if node.Tag == "!!int" || node.Tag == "!!float" {
		return true
	}

	_, err := strconv.ParseFloat(strings.TrimSpace(node.Value), 64)
	return err == nil
}

func sortedNames(set map[string]struct{}) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func diagAtScalarOffset(node *yaml.Node, start, end int, sev protocol.DiagnosticSeverity, msg string) protocol.Diagnostic {
	source := "sdsge-ls"

	line := nodeLine(node)
	col := nodeColumn(node) + max(0, start)
	endCol := nodeColumn(node) + max(max(1, end), start+1)

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

func nodeLine(node *yaml.Node) int {
	if node == nil || node.Line <= 0 {
		return 0
	}
	return node.Line - 1
}

func nodeColumn(node *yaml.Node) int {
	if node == nil || node.Column <= 0 {
		return 0
	}
	return node.Column - 1
}
