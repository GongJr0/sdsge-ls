package validate

import (
	"sort"
	"strings"

	"github.com/GongJr0/sdsge-ls/internal/yamlpos"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"go.yaml.in/yaml/v3"
)

func inferPathFromLines(lines []string, targetLine int) []string {
	type frame struct {
		indent int
		key    string
	}

	var stack []frame

	currentIndent := 0
	if targetLine >= 0 && targetLine < len(lines) {
		currentIndent = countIndent(lines[targetLine])
	}

	for i := 0; i < targetLine && i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			continue
		}

		indent := countIndent(line)
		colon := strings.Index(trimmed, ":")
		if colon < 0 {
			continue
		}

		key := strings.TrimSpace(trimmed[:colon])
		after := strings.TrimSpace(trimmed[colon+1:])

		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}

		// Only container lines become path frames.
		if after == "" || strings.HasPrefix(after, "#") {
			stack = append(stack, frame{indent: indent, key: key})
		}
	}

	// Remove frames that are not ancestors of the current line indent.
	for len(stack) > 0 && stack[len(stack)-1].indent >= currentIndent {
		stack = stack[:len(stack)-1]
	}

	out := make([]string, 0, len(stack))
	for _, f := range stack {
		out = append(out, f.key)
	}
	return out
}

func Complete(text string, line, char int) []protocol.CompletionItem {
	root := parseRoot(text)
	decls := Decls{}
	if root != nil {
		decls = collectDecls(root)
	}

	path, ctx := detectCompletionContext(text, line, char)
	if ctx == nil {
		return nil
	}

	switch ctx.Kind {
	case completionMapKey:
		return completeMapKeys(path, ctx.Prefix, decls)
	case completionSeqValue:
		return completeSeqValues(path, decls, ctx.Prefix)
	case completionMapValue:
		return completeMapValues(path, decls, ctx.Prefix)
	default:
		return nil
	}
}

func parseRoot(text string) *yaml.Node {
	root := yamlpos.ParseBestEffort(text)
	return unwrapDocument(root)
}

type completionKind int

const (
	completionUnknown completionKind = iota
	completionMapKey
	completionMapValue
	completionSeqValue
)

type completionContext struct {
	Kind   completionKind
	Prefix string
}

func countIndent(s string) int {
	n := 0
	for _, ch := range s {
		if ch == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

func detectCompletionContext(text string, line, char int) ([]string, *completionContext) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	if line < 0 || line >= len(lines) {
		return nil, nil
	}

	current := lines[line]
	if char < 0 {
		char = 0
	}
	if char > len(current) {
		char = len(current)
	}

	before := current[:char]
	trimmedBefore := strings.TrimSpace(before)

	// Explicit root blank-line case.
	if trimmedBefore == "" && countIndent(current) == 0 {
		return []string{}, &completionContext{Kind: completionMapKey}
	}

	path := inferPathFromLines(lines, line)

	if strings.HasPrefix(strings.TrimLeft(before, " "), "- ") {
		return path, &completionContext{
			Kind:   completionSeqValue,
			Prefix: seqValuePrefix(before),
		}
	}

	if colon := strings.Index(before, ":"); colon >= 0 {
		return path, &completionContext{
			Kind:   completionMapValue,
			Prefix: strings.TrimSpace(before[colon+1:]),
		}
	}

	if trimmedBefore == "" || !strings.Contains(before, ":") {
		return path, &completionContext{
			Kind:   completionMapKey,
			Prefix: mapKeyPrefix(before),
		}
	}

	return path, &completionContext{Kind: completionUnknown}
}

func completeMapKeys(path []string, prefix string, decls Decls) []protocol.CompletionItem {
	if items := dynamicMapKeyItems(path, decls); items != nil {
		return filterCompletionItems(items, prefix)
	}

	spec := schemaAtPath(path)
	if spec == nil {
		if len(path) == 0 {
			return filterCompletionItems(fieldItems(schema), prefix)
		}
		return nil
	}

	return filterCompletionItems(fieldItems(spec), prefix)
}

func dynamicMapKeyItems(path []string, decls Decls) []protocol.CompletionItem {
	switch pathString(path) {
	case "constrained", "kalman.P0.diag":
		return stringSetItems(decls.Variables, protocol.CompletionItemKindVariable)
	case "calibration.parameters":
		return stringSetItems(decls.Parameters, protocol.CompletionItemKindConstant)
	case "calibration.shocks.std":
		return stringSetItems(decls.Shocks, protocol.CompletionItemKindVariable)
	case "calibration.shocks.corr":
		return pairItemsFromSet(decls.Shocks, protocol.CompletionItemKindVariable)
	case "equations.observables", "kalman.R.std":
		return stringSetItems(decls.Observables, protocol.CompletionItemKindVariable)
	case "kalman.R.corr":
		return pairItemsFromSet(decls.Observables, protocol.CompletionItemKindVariable)
	default:
		return nil
	}
}

func schemaAtPath(path []string) map[string]Field {
	if len(path) == 0 {
		return schema
	}

	if path[0] == "variables" {
		if len(path) == 1 {
			return nil
		}

		current := variableMetadataSchema
		for _, part := range path[2:] {
			field, ok := current[part]
			if !ok {
				return nil
			}
			current = field.Children
			if current == nil {
				return nil
			}
		}
		return current
	}

	current := schema
	for _, part := range path {
		field, ok := current[part]
		if !ok {
			return nil
		}
		current = field.Children
		if current == nil {
			return nil
		}
	}
	return current
}

func fieldItems(spec map[string]Field) []protocol.CompletionItem {
	items := make([]protocol.CompletionItem, 0, len(spec))
	for name, field := range spec {
		items = append(items, protocol.CompletionItem{
			Label:  name,
			Kind:   completionKindField(field),
			Detail: strPtr(fieldDetail(field)),
		})
	}
	return items
}

func filterCompletionItems(items []protocol.CompletionItem, prefix string) []protocol.CompletionItem {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return items
	}

	filtered := make([]protocol.CompletionItem, 0, len(items))
	for _, item := range items {
		if strings.HasPrefix(item.Label, prefix) {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

func completionKindField(field Field) *protocol.CompletionItemKind {
	kind := protocol.CompletionItemKindField
	switch field.NodeKind {
	case yaml.MappingNode:
		kind = protocol.CompletionItemKindModule
	case yaml.SequenceNode:
		kind = protocol.CompletionItemKindProperty
	case yaml.ScalarNode:
		kind = protocol.CompletionItemKindField
	}
	return &kind
}

func fieldDetail(field Field) string {
	if field.Required {
		return "required"
	}
	return "optional"
}

func completeSeqValues(path []string, decls Decls, prefix string) []protocol.CompletionItem {
	switch pathString(path) {
	case "kalman.y":
		return filterCompletionItems(stringSetItems(decls.Observables, protocol.CompletionItemKindVariable), prefix)
	default:
		return nil
	}
}

func completeMapValues(path []string, decls Decls, prefix string) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	switch pathString(path) {
	case "shock_map":
		items = stringSetItems(decls.Variables, protocol.CompletionItemKindVariable)

	case "calibration.shocks.std", "calibration.shocks.corr":
		items = stringSetItems(decls.Parameters, protocol.CompletionItemKindConstant)

	case "equations.observables":
		// completion on RHS expression strings is not worth doing yet
		return nil

	case "constrained":
		items = boolItems()

	case "kalman.R.std", "kalman.R.corr":
		items = stringSetItems(decls.Parameters, protocol.CompletionItemKindConstant)

	case "kalman.P0":
		items = scalarValueItemsForP0()

	case "kalman.P0.diag":
		items = numberStubItems()

	default:
		return nil
	}

	return filterCompletionItems(items, prefix)
}

func pathString(path []string) string {
	return strings.Join(path, ".")
}

func stringSetItems(set map[string]struct{}, kind protocol.CompletionItemKind) []protocol.CompletionItem {
	items := make([]protocol.CompletionItem, 0, len(set))
	for _, name := range sortedStringsFromSet(set) {
		k := kind
		items = append(items, protocol.CompletionItem{
			Label: name,
			Kind:  &k,
		})
	}
	return items
}

func pairItemsFromSet(set map[string]struct{}, kind protocol.CompletionItemKind) []protocol.CompletionItem {
	names := sortedStringsFromSet(set)
	items := make([]protocol.CompletionItem, 0, len(names)*len(names))
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			k := kind
			items = append(items, protocol.CompletionItem{
				Label: names[i] + ", " + names[j],
				Kind:  &k,
			})
		}
	}
	return items
}

func boolItems() []protocol.CompletionItem {
	k := protocol.CompletionItemKindValue
	return []protocol.CompletionItem{
		{Label: "true", Kind: &k},
		{Label: "false", Kind: &k},
	}
}

func scalarValueItemsForP0() []protocol.CompletionItem {
	k := protocol.CompletionItemKindValue
	return []protocol.CompletionItem{
		{Label: "diag", Kind: &k},
		{Label: "eye", Kind: &k},
	}
}

func numberStubItems() []protocol.CompletionItem {
	k := protocol.CompletionItemKindValue
	return []protocol.CompletionItem{
		{Label: "1.0", Kind: &k},
		{Label: "0.0", Kind: &k},
	}
}

func strPtr(s string) *string {
	return &s
}

func mapKeyPrefix(before string) string {
	trimmed := strings.TrimLeft(before, " ")
	if trimmed == "" {
		return ""
	}

	if idx := strings.LastIndex(trimmed, "\t"); idx >= 0 {
		trimmed = trimmed[idx+1:]
	}

	return strings.TrimSpace(trimmed)
}

func seqValuePrefix(before string) string {
	trimmed := strings.TrimLeft(before, " ")
	trimmed = strings.TrimPrefix(trimmed, "-")
	return strings.TrimSpace(trimmed)
}

func sortedStringsFromSet(set map[string]struct{}) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
