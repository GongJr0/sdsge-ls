package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheckReportsValidFixture(t *testing.T) {
	path := filepath.Join("..", "..", "test_configs", "test.model")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"check", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	if got := strings.TrimSpace(stdout.String()); got != path+": no diagnostics" {
		t.Fatalf("unexpected stdout: %q", got)
	}
}

func TestRunCompletePrintsItems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "complete.model")
	text := "observables: [Rate, Infl, OutGap]\nkalman:\n  y:\n    - \n"

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"complete", "--line", "4", "--char", "7", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	got := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	want := []string{
		"Infl\tvariable",
		"OutGap\tvariable",
		"Rate\tvariable",
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d completion lines, got %d: %q", len(want), len(got), stdout.String())
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected line %d to be %q, got %q", i, want[i], got[i])
		}
	}
}

func TestRunCompleteReturnsRootFieldsAfterScalarRootField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "root-after-scalar.model")
	text := "name: \"abcd\"\n"

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"complete", "--line", "2", "--char", "1", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	got := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(got) == 0 {
		t.Fatal("expected root-level completion items, got none")
	}

	wantPrefix := []string{
		"calibration\tmodule\trequired",
		"constrained\tmodule\toptional",
		"equations\tmodule\trequired",
	}

	for i := range wantPrefix {
		if got[i] != wantPrefix[i] {
			t.Fatalf("expected line %d to be %q, got %q", i, wantPrefix[i], got[i])
		}
	}
}

func TestRunCompleteReturnsRootFieldsAfterScalarRootFieldCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "root-after-scalar-crlf.model")
	text := "name: \"abcd\"\r\n"

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"complete", "--line", "2", "--char", "1", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	got := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(got) == 0 {
		t.Fatal("expected root-level completion items for CRLF input, got none")
	}

	wantPrefix := []string{
		"calibration\tmodule\trequired",
		"constrained\tmodule\toptional",
		"equations\tmodule\trequired",
	}

	for i := range wantPrefix {
		if got[i] != wantPrefix[i] {
			t.Fatalf("expected line %d to be %q, got %q", i, wantPrefix[i], got[i])
		}
	}
}

func TestRunCompleteReturnsPrefixMatchedRootFieldDespiteParseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "root-after-scalar-prefix.model")
	text := "name: \"abcd\"\nkal"

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"complete", "--line", "2", "--char", "4", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	got := strings.TrimSpace(stdout.String())
	want := "kalman\tmodule\toptional"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRunDefinitionPrintsParameterDeclaration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xref.model")
	text := strings.Join([]string{
		"name: demo",
		"variables: [x]",
		"parameters: [beta, meas_infl]",
		"shock_map: {}",
		"observables: [Infl]",
		"equations:",
		"  model:",
		"    - x(t) = beta*x(t+1)",
		"  constraint: {}",
		"  observables:",
		"    Infl: beta + 1",
		"calibration:",
		"  parameters:",
		"    beta: 0.9",
		"    meas_infl: 1.0",
		"  shocks:",
		"    std: {}",
		"kalman:",
		"  y: [Infl]",
		"  R:",
		"    std:",
		"      Infl: meas_infl",
	}, "\n") + "\n"

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"definition", "--line", "22", "--char", "14", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	got := strings.TrimSpace(stdout.String())
	want := path + ":3:20: declaration parameter meas_infl"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRunReferencesPrintsObservableReferences(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xref-refs.model")
	text := strings.Join([]string{
		"name: demo",
		"variables: [x]",
		"parameters: [beta, meas_infl]",
		"shock_map: {}",
		"observables: [Infl]",
		"equations:",
		"  model:",
		"    - x(t) = beta*x(t+1)",
		"  constraint: {}",
		"  observables:",
		"    Infl: beta + 1",
		"calibration:",
		"  parameters:",
		"    beta: 0.9",
		"    meas_infl: 1.0",
		"  shocks:",
		"    std: {}",
		"kalman:",
		"  y: [Infl]",
		"  R:",
		"    std:",
		"      Infl: meas_infl",
	}, "\n") + "\n"

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"references", "--line", "5", "--char", "15", "--include-declaration", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	got := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	want := []string{
		path + ":5:15: declaration observable Infl",
		path + ":11:5: reference observable Infl",
		path + ":19:7: reference observable Infl",
		path + ":22:7: reference observable Infl",
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d reference lines, got %d: %q", len(want), len(got), stdout.String())
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected line %d to be %q, got %q", i, want[i], got[i])
		}
	}
}
