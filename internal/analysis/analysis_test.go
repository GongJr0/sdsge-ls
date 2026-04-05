package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckValidFixtureHasNoDiagnostics(t *testing.T) {
	path := filepath.Join("..", "..", "test_configs", "test.model")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	diags := Check(string(data))
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %d: %#v", len(diags), diags)
	}
}

func TestCheckInvalidDocumentReportsErrors(t *testing.T) {
	text := strings.TrimSpace(`
name: demo
variables: [x]
parameters: [beta]
shock_map:
  e_x: y
observables: [Obs]
equations:
  model:
    - x(t) = beta*x(t+1) + e_x
  observables:
    Obs: x(t)
calibration:
  parameters:
    beta: 0.9
  shocks:
    std:
      e_x: beta
`)

	diags := Check(text)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for invalid shock_map reference")
	}
	if !HasErrors(diags) {
		t.Fatal("expected at least one error diagnostic")
	}
}

func TestCheckValidMappingVariablesHasNoDiagnostics(t *testing.T) {
	text := strings.TrimSpace(`
name: demo
variables:
  x:
    linearization: taylor
    steady_state: beta
  r:
    linearization: log
    steady_state: r_star
parameters: [beta, r_star]
shock_map:
  e_x: x
observables: [Obs]
equations:
  model:
    - x(t) = beta*x(t+1) + e_x
  constraint: {}
  observables:
    Obs: x(t) + r_star
calibration:
  parameters:
    beta: 0.9
    r_star: 1.0
  shocks:
    std:
      e_x: beta
    corr: {}
kalman:
  y: [Obs]
  R:
    std:
      Obs: beta
    corr: {}
`) + "\n"

	diags := Check(text)
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics for mapping-style variables, got %#v", diags)
	}
}

func TestCompleteSortsItems(t *testing.T) {
	text := "kalman:\n  y:\n    - \nobservables: [Rate, Infl, OutGap]\n"

	items := Complete(text, 2, 6)
	if len(items) != 3 {
		t.Fatalf("expected 3 completion items, got %d", len(items))
	}

	want := []string{"Infl", "OutGap", "Rate"}
	for i, label := range want {
		if items[i].Label != label {
			t.Fatalf("expected item %d to be %q, got %q", i, label, items[i].Label)
		}
	}
}

func TestCompleteReturnsMapKeySuggestionsDespiteParseError(t *testing.T) {
	text := "name: \"abcd\"\nkal"

	items := Complete(text, 1, 3)
	if len(items) != 1 {
		t.Fatalf("expected exactly one completion item, got %d: %#v", len(items), items)
	}

	if items[0].Label != "kalman" {
		t.Fatalf("expected completion item to be %q, got %q", "kalman", items[0].Label)
	}
}

func TestCompleteReturnsDeclarationBackedKeysDespiteParseError(t *testing.T) {
	text := strings.Join([]string{
		"name: demo",
		"variables: [x]",
		"parameters: [beta, gamma]",
		"shock_map:",
		"  e_x: x",
		"observables: [Obs]",
		"equations:",
		"  model:",
		"    - x(t) = beta*x(t+1) + e_x",
		"  constraint: {}",
		"  observables:",
		"    Obs: x(t)",
		"calibration:",
		"  parameters:",
		"    be",
	}, "\n") + "\n"

	items := Complete(text, 14, 6)
	if len(items) != 1 || items[0].Label != "beta" {
		t.Fatalf("expected beta completion from declaration-backed key context, got %#v", items)
	}
}

func TestCompleteReturnsObservablePairKeys(t *testing.T) {
	text := strings.Join([]string{
		"name: demo",
		"variables: [x]",
		"parameters: [rho_ir]",
		"shock_map:",
		"  e_x: x",
		"observables: [Infl, Rate]",
		"equations:",
		"  model:",
		"    - x(t) = x(t+1) + e_x",
		"  constraint: {}",
		"  observables:",
		"    Infl: x(t)",
		"    Rate: x(t)",
		"calibration:",
		"  parameters:",
		"    rho_ir: 0.0",
		"  shocks:",
		"    std:",
		"      e_x: rho_ir",
		"kalman:",
		"  y: [Infl, Rate]",
		"  R:",
		"    std:",
		"      Infl: rho_ir",
		"      Rate: rho_ir",
		"    corr:",
		"      In",
	}, "\n") + "\n"

	items := Complete(text, 26, 8)
	if len(items) != 1 || items[0].Label != "Infl, Rate" {
		t.Fatalf("expected observable pair completion, got %#v", items)
	}
}

func TestCompleteReturnsVariableMetadataKeys(t *testing.T) {
	text := strings.Join([]string{
		"name: demo",
		"variables:",
		"  x:",
		"    ",
	}, "\n")

	items := Complete(text, 3, 4)
	if len(items) != 2 {
		t.Fatalf("expected 2 variable metadata completion items, got %#v", items)
	}

	want := []string{"linearization", "steady_state"}
	for i, label := range want {
		if items[i].Label != label {
			t.Fatalf("expected item %d to be %q, got %#v", i, label, items)
		}
	}
}

func TestCheckReportsPairAndEquationIdentifierErrors(t *testing.T) {
	text := strings.Join([]string{
		"name: demo",
		"variables: [x]",
		"parameters: [beta]",
		"shock_map:",
		"  e_x: x",
		"  e_z: x",
		"observables: [Obs]",
		"equations:",
		"  model:",
		"    - z(t) = alpha*x(t+1) + e_y",
		"  constraint: {}",
		"  observables:",
		"    Obs: x(t)",
		"calibration:",
		"  parameters:",
		"    beta: nope",
		"  shocks:",
		"    std:",
		"      e_x: beta",
		"    corr:",
		"      e_x, e_x: beta",
		"kalman:",
		"  y: [Obs]",
		"  R:",
		"    std:",
		"      Obs: beta",
		"    corr:",
		"      Obs, Missing: beta",
	}, "\n") + "\n"

	diags := Check(text)
	assertHasDiagnostic(t, diags, `left-hand side variable "z" is not declared in variables`)
	assertHasDiagnostic(t, diags, `unknown identifier "alpha" in equations.model`)
	assertHasDiagnostic(t, diags, `unknown identifier "e_y" in equations.model`)
	assertHasDiagnostic(t, diags, `declared shock "e_z" is missing from calibration.shocks.std`)
	assertHasDiagnostic(t, diags, `key "e_x, e_x" in calibration.shocks.corr must refer to two distinct names`)
	assertHasDiagnostic(t, diags, `name "Missing" in kalman.R.corr is not declared in observables`)
	assertHasDiagnostic(t, diags, `value for calibration.parameters.beta must be numeric`)
}

func TestCheckRejectsInvalidVariableLinearizationMethod(t *testing.T) {
	text := strings.TrimSpace(`
name: demo
variables:
  x:
    linearization: quadratic
parameters: [beta]
shock_map:
  e_x: x
observables: [Obs]
equations:
  model:
    - x(t) = x(t+1) + e_x
  constraint: {}
  observables:
    Obs: x(t)
calibration:
  parameters:
    beta: 0.9
  shocks:
    std:
      e_x: beta
    corr: {}
kalman:
  y: [Obs]
  R:
    std:
      Obs: beta
    corr: {}
`) + "\n"

	diags := Check(text)
	assertHasDiagnostic(t, diags, `variables.x.linearization must be one of: log, none, taylor`)
}

func assertHasDiagnostic(t *testing.T, diags []Diagnostic, msg string) {
	t.Helper()

	for _, diag := range diags {
		if diag.Message == msg {
			return
		}
	}

	t.Fatalf("expected diagnostic %q, got %#v", msg, diags)
}
