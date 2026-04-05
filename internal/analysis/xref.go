package analysis

import (
	"sort"
	"strings"

	"github.com/GongJr0/sdsge-ls/internal/expr"
	"github.com/GongJr0/sdsge-ls/internal/yamlpos"
	"go.yaml.in/yaml/v3"
)

type symbolKey struct {
	Name string
	Kind SymbolKind
}

type symbolIndex struct {
	declarations map[symbolKey]SymbolOccurrence
	occurrences  map[symbolKey][]SymbolOccurrence
	flat         []SymbolOccurrence
}

func Definition(text string, line, char int) *SymbolOccurrence {
	index := buildSymbolIndex(text)
	if index == nil {
		return nil
	}

	occurrence, ok := index.lookupOccurrence(text, line, char)
	if !ok {
		return nil
	}

	key := symbolKey{Name: occurrence.Name, Kind: occurrence.Kind}
	declaration, ok := index.declarations[key]
	if !ok {
		return nil
	}

	return &declaration
}

func References(text string, line, char int, includeDeclaration bool) []SymbolOccurrence {
	index := buildSymbolIndex(text)
	if index == nil {
		return nil
	}

	occurrence, ok := index.lookupOccurrence(text, line, char)
	if !ok {
		return nil
	}

	key := symbolKey{Name: occurrence.Name, Kind: occurrence.Kind}
	items := index.occurrences[key]
	if len(items) == 0 {
		return nil
	}

	out := make([]SymbolOccurrence, 0, len(items))
	for _, item := range items {
		if !includeDeclaration && item.Role == SymbolRoleDeclaration {
			continue
		}
		out = append(out, item)
	}

	return out
}

func buildSymbolIndex(text string) *symbolIndex {
	root := yamlpos.ParseBestEffort(text)
	if root == nil {
		return nil
	}

	doc := unwrapDocument(root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return nil
	}

	index := &symbolIndex{
		declarations: map[symbolKey]SymbolOccurrence{},
		occurrences:  map[symbolKey][]SymbolOccurrence{},
	}

	index.collectDeclaredVariables(getMapValue(doc, "variables"))
	index.collectDeclaredSequence(getMapValue(doc, "parameters"), SymbolKindParameter)
	index.collectDeclaredSequence(getMapValue(doc, "observables"), SymbolKindObservable)
	index.collectDeclaredMappingKeys(getMapValue(doc, "shock_map"), SymbolKindShock)

	index.collectVariableReferences(doc)
	index.collectParameterReferences(doc)
	index.collectObservableReferences(doc)
	index.collectShockReferences(doc)
	index.sort()

	return index
}

func (s *symbolIndex) collectDeclaredVariables(node *yaml.Node) {
	switch {
	case node == nil:
		return
	case node.Kind == yaml.SequenceNode:
		s.collectDeclaredSequence(node, SymbolKindVariable)
	case node.Kind == yaml.MappingNode:
		s.collectDeclaredMappingKeys(node, SymbolKindVariable)
	}
}

func (s *symbolIndex) collectDeclaredSequence(node *yaml.Node, kind SymbolKind) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return
	}

	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			continue
		}

		s.add(occurrenceAtNode(item.Value, kind, SymbolRoleDeclaration, item))
	}
}

func (s *symbolIndex) collectDeclaredMappingKeys(node *yaml.Node, kind SymbolKind) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}

		s.add(occurrenceAtNode(keyNode.Value, kind, SymbolRoleDeclaration, keyNode))
	}
}

func (s *symbolIndex) collectVariableReferences(doc *yaml.Node) {
	s.collectMappingKeyReferences(getMapValue(doc, "constrained"), SymbolKindVariable)
	s.collectMappingValueReferences(getMapValue(doc, "shock_map"), SymbolKindVariable)
	s.collectMappingKeyReferences(getNestedMapValue(doc, "kalman", "P0", "diag"), SymbolKindVariable)
	s.collectVariableSteadyStateReferences(getMapValue(doc, "variables"), SymbolKindVariable)
	s.collectExpressionReferences(getNestedMapValue(doc, "equations", "model"), SymbolKindVariable)
	s.collectExpressionReferences(getNestedMapValue(doc, "equations", "observables"), SymbolKindVariable)
}

func (s *symbolIndex) collectParameterReferences(doc *yaml.Node) {
	s.collectMappingKeyReferences(getNestedMapValue(doc, "calibration", "parameters"), SymbolKindParameter)
	s.collectMappingValueReferences(getNestedMapValue(doc, "calibration", "shocks", "std"), SymbolKindParameter)
	s.collectMappingValueReferences(getNestedMapValue(doc, "calibration", "shocks", "corr"), SymbolKindParameter)
	s.collectMappingValueReferences(getNestedMapValue(doc, "kalman", "R", "std"), SymbolKindParameter)
	s.collectMappingValueReferences(getNestedMapValue(doc, "kalman", "R", "corr"), SymbolKindParameter)
	s.collectVariableSteadyStateReferences(getMapValue(doc, "variables"), SymbolKindParameter)
	s.collectExpressionReferences(getNestedMapValue(doc, "equations", "model"), SymbolKindParameter)
	s.collectExpressionReferences(getNestedMapValue(doc, "equations", "observables"), SymbolKindParameter)
}

func (s *symbolIndex) collectObservableReferences(doc *yaml.Node) {
	s.collectMappingKeyReferences(getNestedMapValue(doc, "equations", "observables"), SymbolKindObservable)
	s.collectSequenceValueReferences(getNestedMapValue(doc, "kalman", "y"), SymbolKindObservable)
	s.collectMappingKeyReferences(getNestedMapValue(doc, "kalman", "R", "std"), SymbolKindObservable)
	s.collectPairKeyReferences(getNestedMapValue(doc, "kalman", "R", "corr"), SymbolKindObservable)
}

func (s *symbolIndex) collectShockReferences(doc *yaml.Node) {
	s.collectMappingKeyReferences(getNestedMapValue(doc, "calibration", "shocks", "std"), SymbolKindShock)
	s.collectPairKeyReferences(getNestedMapValue(doc, "calibration", "shocks", "corr"), SymbolKindShock)
	s.collectVariableSteadyStateReferences(getMapValue(doc, "variables"), SymbolKindShock)
	s.collectExpressionReferences(getNestedMapValue(doc, "equations", "model"), SymbolKindShock)
	s.collectExpressionReferences(getNestedMapValue(doc, "equations", "observables"), SymbolKindShock)
}

func (s *symbolIndex) collectVariableSteadyStateReferences(node *yaml.Node, kind SymbolKind) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		specNode := node.Content[i+1]
		if specNode == nil || specNode.Kind != yaml.MappingNode {
			continue
		}

		steadyStateNode := getMapValue(specNode, "steady_state")
		if steadyStateNode == nil || steadyStateNode.Kind != yaml.ScalarNode {
			continue
		}

		s.collectExpressionScalar(steadyStateNode, kind)
	}
}

func (s *symbolIndex) collectMappingKeyReferences(node *yaml.Node, kind SymbolKind) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}

		key := symbolKey{Name: keyNode.Value, Kind: kind}
		if _, ok := s.declarations[key]; !ok {
			continue
		}

		s.add(occurrenceAtNode(keyNode.Value, kind, SymbolRoleReference, keyNode))
	}
}

func (s *symbolIndex) collectPairKeyReferences(node *yaml.Node, kind SymbolKind) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}

		for _, ident := range expr.FindIdentifiers(keyNode.Value) {
			key := symbolKey{Name: ident.Name, Kind: kind}
			if _, ok := s.declarations[key]; !ok {
				continue
			}

			s.add(occurrenceAtOffset(ident.Name, kind, SymbolRoleReference, keyNode, ident.Start, ident.End))
		}
	}
}

func (s *symbolIndex) collectMappingValueReferences(node *yaml.Node, kind SymbolKind) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	for i := 1; i < len(node.Content); i += 2 {
		valNode := node.Content[i]
		if valNode.Kind != yaml.ScalarNode {
			continue
		}

		key := symbolKey{Name: valNode.Value, Kind: kind}
		if _, ok := s.declarations[key]; !ok {
			continue
		}

		s.add(occurrenceAtNode(valNode.Value, kind, SymbolRoleReference, valNode))
	}
}

func (s *symbolIndex) collectSequenceValueReferences(node *yaml.Node, kind SymbolKind) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return
	}

	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode {
			continue
		}

		key := symbolKey{Name: item.Value, Kind: kind}
		if _, ok := s.declarations[key]; !ok {
			continue
		}

		s.add(occurrenceAtNode(item.Value, kind, SymbolRoleReference, item))
	}
}

func (s *symbolIndex) collectExpressionReferences(node *yaml.Node, kind SymbolKind) {
	switch {
	case node == nil:
		return
	case node.Kind == yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind == yaml.ScalarNode {
				s.collectExpressionScalar(item, kind)
			}
		}
	case node.Kind == yaml.MappingNode:
		for i := 1; i < len(node.Content); i += 2 {
			valNode := node.Content[i]
			if valNode.Kind == yaml.ScalarNode {
				s.collectExpressionScalar(valNode, kind)
			}
		}
	}
}

func (s *symbolIndex) collectExpressionScalar(node *yaml.Node, kind SymbolKind) {
	for _, ident := range expr.FindIdentifiers(node.Value) {
		if !matchesSymbolKind(ident, kind) {
			continue
		}

		key := symbolKey{Name: ident.Name, Kind: kind}
		if _, ok := s.declarations[key]; !ok {
			continue
		}

		s.add(occurrenceAtOffset(ident.Name, kind, SymbolRoleReference, node, ident.Start, ident.End))
	}
}

func matchesSymbolKind(ident expr.Identifier, kind SymbolKind) bool {
	switch kind {
	case SymbolKindVariable:
		return ident.TimeIndexed || (!ident.Function && !ident.TimeIndexed)
	case SymbolKindParameter:
		return !ident.Function && !ident.TimeIndexed
	case SymbolKindShock:
		return !ident.Function && !ident.TimeIndexed
	default:
		return false
	}
}

func (s *symbolIndex) add(occurrence SymbolOccurrence) {
	key := symbolKey{Name: occurrence.Name, Kind: occurrence.Kind}

	if occurrence.Role == SymbolRoleDeclaration {
		if _, exists := s.declarations[key]; !exists {
			s.declarations[key] = occurrence
		}
	}

	s.occurrences[key] = append(s.occurrences[key], occurrence)
	s.flat = append(s.flat, occurrence)
}

func (s *symbolIndex) sort() {
	sort.Slice(s.flat, func(i, j int) bool {
		return occurrenceLess(s.flat[i], s.flat[j])
	})

	for key, items := range s.occurrences {
		sort.Slice(items, func(i, j int) bool {
			return occurrenceLess(items[i], items[j])
		})
		s.occurrences[key] = items
	}
}

func (s *symbolIndex) lookupOccurrence(text string, line, char int) (SymbolOccurrence, bool) {
	if occurrence, ok := s.occurrenceAt(line, char); ok {
		return occurrence, true
	}

	word := wordAtPosition(text, line, char)
	if word == "" {
		return SymbolOccurrence{}, false
	}

	kinds := s.kindsForName(word)
	if len(kinds) != 1 {
		return SymbolOccurrence{}, false
	}

	return SymbolOccurrence{Name: word, Kind: kinds[0], Role: SymbolRoleReference}, true
}

func (s *symbolIndex) kindsForName(name string) []SymbolKind {
	out := make([]SymbolKind, 0, 4)
	for _, kind := range []SymbolKind{SymbolKindVariable, SymbolKindParameter, SymbolKindObservable, SymbolKindShock} {
		if _, ok := s.declarations[symbolKey{Name: name, Kind: kind}]; ok {
			out = append(out, kind)
		}
	}
	return out
}

func (s *symbolIndex) occurrenceAt(line, char int) (SymbolOccurrence, bool) {
	for _, occurrence := range s.flat {
		if positionInRange(line, char, occurrence.Range) {
			return occurrence, true
		}
	}

	return SymbolOccurrence{}, false
}

func wordAtPosition(text string, line, char int) string {
	lines := splitLines(text)
	if line < 0 || line >= len(lines) {
		return ""
	}

	current := lines[line]
	if char < 0 {
		char = 0
	}
	if char > len(current) {
		char = len(current)
	}

	start := char
	for start > 0 && isIdentChar(current[start-1]) {
		start--
	}

	end := char
	for end < len(current) && isIdentChar(current[end]) {
		end++
	}

	if start == end {
		return ""
	}

	word := current[start:end]
	if word == "t" {
		return ""
	}
	return word
}

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

func isIdentChar(ch byte) bool {
	return ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9')
}

func occurrenceLess(left, right SymbolOccurrence) bool {
	if left.Range.Start.Line != right.Range.Start.Line {
		return left.Range.Start.Line < right.Range.Start.Line
	}
	if left.Range.Start.Character != right.Range.Start.Character {
		return left.Range.Start.Character < right.Range.Start.Character
	}
	if left.Role != right.Role {
		return left.Role < right.Role
	}
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	return left.Name < right.Name
}

func positionInRange(line, char int, r Range) bool {
	if line < r.Start.Line || line > r.End.Line {
		return false
	}

	if line == r.Start.Line && char < r.Start.Character {
		return false
	}

	if line == r.End.Line && char >= r.End.Character {
		return false
	}

	return true
}

func occurrenceAtNode(name string, kind SymbolKind, role SymbolRole, node *yaml.Node) SymbolOccurrence {
	return occurrenceWithCoordinates(name, kind, role, nodeLine(node), nodeColumn(node), len(name))
}

func occurrenceAtOffset(name string, kind SymbolKind, role SymbolRole, node *yaml.Node, startOffset, endOffset int) SymbolOccurrence {
	return occurrenceWithCoordinates(
		name,
		kind,
		role,
		nodeLine(node),
		nodeColumn(node)+startOffset,
		endOffset-startOffset,
	)
}

func occurrenceWithCoordinates(name string, kind SymbolKind, role SymbolRole, line, char, width int) SymbolOccurrence {
	if width < 1 {
		width = 1
	}

	return SymbolOccurrence{
		Name: name,
		Kind: kind,
		Role: role,
		Range: Range{
			Start: Position{Line: line, Character: char},
			End:   Position{Line: line, Character: char + width},
		},
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

func unwrapDocument(root *yaml.Node) *yaml.Node {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		return root.Content[0]
	}
	return root
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
