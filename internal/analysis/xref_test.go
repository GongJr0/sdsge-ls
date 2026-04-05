package analysis

import (
	"strings"
	"testing"
)

func TestDefinitionFindsTopLevelParameterDeclaration(t *testing.T) {
	text := xrefFixture()

	line, char := findOccurrence(t, text, "meas_infl", 2)
	definition := Definition(text, line, char)
	if definition == nil {
		t.Fatal("expected a definition for meas_infl")
	}

	if definition.Name != "meas_infl" {
		t.Fatalf("expected definition name %q, got %q", "meas_infl", definition.Name)
	}
	if definition.Kind != SymbolKindParameter {
		t.Fatalf("expected symbol kind %q, got %q", SymbolKindParameter, definition.Kind)
	}
	if definition.Role != SymbolRoleDeclaration {
		t.Fatalf("expected declaration role, got %q", definition.Role)
	}
}

func TestReferencesReturnObservableUsages(t *testing.T) {
	text := xrefFixture()

	line, char := findOccurrence(t, text, "Infl", 0)
	references := References(text, line, char, true)

	if len(references) != 4 {
		t.Fatalf("expected 4 observable occurrences including declaration, got %d: %#v", len(references), references)
	}

	if references[0].Role != SymbolRoleDeclaration {
		t.Fatalf("expected first occurrence to be a declaration, got %q", references[0].Role)
	}

	for _, occurrence := range references {
		if occurrence.Name != "Infl" {
			t.Fatalf("expected all occurrences to be for %q, got %#v", "Infl", references)
		}
		if occurrence.Kind != SymbolKindObservable {
			t.Fatalf("expected all occurrences to be observable symbols, got %#v", references)
		}
	}
}

func TestReferencesReturnParameterEquationUsages(t *testing.T) {
	text := xrefFixture()

	line, char := findOccurrence(t, text, "beta", 0)
	references := References(text, line, char, false)
	if len(references) != 4 {
		t.Fatalf("expected 4 beta references without declaration, got %d: %#v", len(references), references)
	}
}

func TestReferencesReturnVariableUsages(t *testing.T) {
	text := xrefFixture()

	line, char := findOccurrence(t, text, "x", 0)
	references := References(text, line, char, true)
	if len(references) != 4 {
		t.Fatalf("expected 4 x occurrences including declaration, got %d: %#v", len(references), references)
	}

	if references[0].Kind != SymbolKindVariable {
		t.Fatalf("expected variable symbol kind, got %#v", references)
	}
}

func TestDefinitionFindsShockDeclarationFromModelEquation(t *testing.T) {
	text := xrefFixture()

	line, char := findOccurrence(t, text, "e_x", 2)
	definition := Definition(text, line, char)
	if definition == nil {
		t.Fatal("expected a definition for e_x")
	}

	if definition.Kind != SymbolKindShock {
		t.Fatalf("expected shock symbol kind, got %q", definition.Kind)
	}
	if definition.Role != SymbolRoleDeclaration {
		t.Fatalf("expected declaration role, got %q", definition.Role)
	}
}

func TestDefinitionFallsBackOnIncompleteDocumentForUniqueSymbol(t *testing.T) {
	text := "parameters: [meas_infl]\nmeas_infl"

	line, char := findOccurrence(t, text, "meas_infl", 1)
	definition := Definition(text, line, char)
	if definition == nil {
		t.Fatal("expected a definition on an incomplete document")
	}

	if definition.Name != "meas_infl" || definition.Kind != SymbolKindParameter {
		t.Fatalf("expected parameter definition for meas_infl, got %#v", definition)
	}
}

func TestDefinitionFindsVariableMappingDeclaration(t *testing.T) {
	text := strings.Join([]string{
		"name: demo",
		"variables:",
		"  x:",
		"    steady_state: beta",
		"parameters: [beta, meas_infl]",
		"shock_map:",
		"  e_x: x",
		"observables: [Infl]",
		"equations:",
		"  model:",
		"    - x(t) = x(t+1) + e_x",
		"  constraint: {}",
		"  observables:",
		"    Infl: x(t)",
		"calibration:",
		"  parameters:",
		"    beta: 0.9",
		"    meas_infl: 1.0",
		"  shocks:",
		"    std:",
		"      e_x: beta",
		"kalman:",
		"  y: [Infl]",
		"  R:",
		"    std:",
		"      Infl: meas_infl",
	}, "\n") + "\n"

	line, char := findOccurrence(t, text, "x(t) = x(t+1) + e_x", 0)
	definition := Definition(text, line, char)
	if definition == nil {
		t.Fatal("expected a definition for x")
	}
	if definition.Name != "x" || definition.Kind != SymbolKindVariable {
		t.Fatalf("expected variable definition for x, got %#v", definition)
	}
	if definition.Role != SymbolRoleDeclaration {
		t.Fatalf("expected declaration role, got %#v", definition)
	}
}

func TestReferencesIncludeSteadyStateParameterUsage(t *testing.T) {
	text := strings.Join([]string{
		"name: demo",
		"variables:",
		"  x:",
		"    steady_state: beta",
		"parameters: [beta, meas_infl]",
		"shock_map:",
		"  e_x: x",
		"observables: [Infl]",
		"equations:",
		"  model:",
		"    - x(t) = x(t+1) + e_x",
		"  constraint: {}",
		"  observables:",
		"    Infl: x(t)",
		"calibration:",
		"  parameters:",
		"    beta: 0.9",
		"    meas_infl: 1.0",
		"  shocks:",
		"    std:",
		"      e_x: beta",
		"kalman:",
		"  y: [Infl]",
		"  R:",
		"    std:",
		"      Infl: meas_infl",
	}, "\n") + "\n"

	line, char := findOccurrence(t, text, "beta", 0)
	references := References(text, line, char, false)
	if len(references) != 3 {
		t.Fatalf("expected 3 beta references without declaration, got %d: %#v", len(references), references)
	}
}

func xrefFixture() string {
	return strings.Join([]string{
		"name: demo",
		"variables: [x]",
		"parameters: [beta, meas_infl]",
		"shock_map:",
		"  e_x: x",
		"observables: [Infl]",
		"equations:",
		"  model:",
		"    - x(t) = beta*x(t+1) + e_x",
		"  constraint: {}",
		"  observables:",
		"    Infl: beta + 1",
		"calibration:",
		"  parameters:",
		"    beta: 0.9",
		"    meas_infl: 1.0",
		"  shocks:",
		"    std:",
		"      e_x: beta",
		"kalman:",
		"  y: [Infl]",
		"  R:",
		"    std:",
		"      Infl: meas_infl",
	}, "\n") + "\n"
}

func findOccurrence(t *testing.T, text, needle string, occurrence int) (int, int) {
	t.Helper()

	start := 0
	for i := 0; i <= occurrence; i++ {
		index := strings.Index(text[start:], needle)
		if index < 0 {
			t.Fatalf("could not find occurrence %d of %q", occurrence, needle)
		}
		start += index
		if i == occurrence {
			return lineAndCharAt(text, start)
		}
		start += len(needle)
	}

	t.Fatalf("unreachable")
	return 0, 0
}

func lineAndCharAt(text string, offset int) (int, int) {
	line := 0
	char := 0

	for i := 0; i < offset && i < len(text); i++ {
		if text[i] == '\n' {
			line++
			char = 0
			continue
		}
		char++
	}

	return line, char
}
