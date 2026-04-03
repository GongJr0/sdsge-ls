package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/GongJr0/sdsge-ls/internal/analysis"
	"github.com/GongJr0/sdsge-ls/internal/logging"
)

const lsName = "sdsge-ls"

var version = "0.0.1"

type globalOptions struct {
	LogLevel string
	LogFile  string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, command, commandArgs, err := parseGlobalOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		printUsage(stderr)
		return 2
	}

	if command == "help" {
		printUsage(stdout)
		return 0
	}
	if command == "version" {
		fmt.Fprintln(stdout, version)
		return 0
	}

	logger, closer, err := logging.New(opts.LogLevel, opts.LogFile)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	defer closer.Close()

	switch command {
	case "", "serve":
		return runServer(logger)
	case "check":
		return runCheck(commandArgs, stdout, stderr, logger)
	case "complete":
		return runComplete(commandArgs, stdout, stderr, logger)
	case "definition":
		return runDefinition(commandArgs, stdout, stderr, logger)
	case "references", "refs":
		return runReferences(commandArgs, stdout, stderr, logger)
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n\n", command)
		printUsage(stderr)
		return 2
	}
}

func parseGlobalOptions(args []string) (globalOptions, string, []string, error) {
	var opts globalOptions

	fs := flag.NewFlagSet(lsName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.LogLevel, "log-level", valueOrDefault(os.Getenv("SDSGE_LS_LOG_LEVEL"), "warn"), "log level")
	fs.StringVar(&opts.LogFile, "log-file", os.Getenv("SDSGE_LS_LOG_FILE"), "log file path")

	if err := fs.Parse(args); err != nil {
		return globalOptions{}, "", nil, err
	}

	rest := fs.Args()
	if len(rest) == 0 {
		return opts, "", nil, nil
	}

	return opts, normalizeCommand(rest[0]), rest[1:], nil
}

func normalizeCommand(command string) string {
	switch command {
	case "-h", "--help":
		return "help"
	case "-v", "--version":
		return "version"
	default:
		return command
	}
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func runCheck(args []string, stdout, stderr io.Writer, logger *slog.Logger) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		printUsage(stderr)
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: check expects exactly one file path")
		fmt.Fprintln(stderr)
		printUsage(stderr)
		return 2
	}

	path := fs.Arg(0)
	text, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: read %s: %v\n", path, err)
		return 1
	}

	logger.Info("checking file", "path", path)
	diags := analysis.Check(string(text))

	if len(diags) == 0 {
		fmt.Fprintf(stdout, "%s: no diagnostics\n", path)
		return 0
	}

	for _, diag := range diags {
		fmt.Fprintln(stdout, formatDiagnostic(path, diag))
	}

	if analysis.HasErrors(diags) {
		return 1
	}
	return 0
}

func runComplete(args []string, stdout, stderr io.Writer, logger *slog.Logger) int {
	fs := flag.NewFlagSet("complete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	line := fs.Int("line", 1, "1-based line")
	char := fs.Int("char", 1, "1-based character")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		printUsage(stderr)
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: complete expects exactly one file path")
		fmt.Fprintln(stderr)
		printUsage(stderr)
		return 2
	}
	if *line < 1 || *char < 1 {
		fmt.Fprintln(stderr, "error: --line and --char must be 1-based positive integers")
		return 2
	}

	path := fs.Arg(0)
	text, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: read %s: %v\n", path, err)
		return 1
	}

	zeroBasedLine := *line - 1
	zeroBasedChar := *char - 1

	logger.Info("computing completions", "path", path, "line", *line, "char", *char)
	items := analysis.Complete(string(text), zeroBasedLine, zeroBasedChar)

	if len(items) == 0 {
		fmt.Fprintln(stdout, "no completion items")
		return 0
	}

	for _, item := range items {
		fmt.Fprintln(stdout, formatCompletionItem(item))
	}

	return 0
}

func runDefinition(args []string, stdout, stderr io.Writer, logger *slog.Logger) int {
	fs := flag.NewFlagSet("definition", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	line := fs.Int("line", 1, "1-based line")
	char := fs.Int("char", 1, "1-based character")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		printUsage(stderr)
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: definition expects exactly one file path")
		fmt.Fprintln(stderr)
		printUsage(stderr)
		return 2
	}
	if *line < 1 || *char < 1 {
		fmt.Fprintln(stderr, "error: --line and --char must be 1-based positive integers")
		return 2
	}

	path := fs.Arg(0)
	text, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: read %s: %v\n", path, err)
		return 1
	}

	logger.Info("finding definition", "path", path, "line", *line, "char", *char)
	occurrence := analysis.Definition(string(text), *line-1, *char-1)
	if occurrence == nil {
		fmt.Fprintln(stdout, "no definition found")
		return 0
	}

	fmt.Fprintln(stdout, formatSymbolOccurrence(path, *occurrence))
	return 0
}

func runReferences(args []string, stdout, stderr io.Writer, logger *slog.Logger) int {
	fs := flag.NewFlagSet("references", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	line := fs.Int("line", 1, "1-based line")
	char := fs.Int("char", 1, "1-based character")
	includeDeclaration := fs.Bool("include-declaration", false, "include the symbol declaration in results")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "error: %v\n\n", err)
		printUsage(stderr)
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: references expects exactly one file path")
		fmt.Fprintln(stderr)
		printUsage(stderr)
		return 2
	}
	if *line < 1 || *char < 1 {
		fmt.Fprintln(stderr, "error: --line and --char must be 1-based positive integers")
		return 2
	}

	path := fs.Arg(0)
	text, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: read %s: %v\n", path, err)
		return 1
	}

	logger.Info(
		"finding references",
		"path", path,
		"line", *line,
		"char", *char,
		"includeDeclaration", *includeDeclaration,
	)
	occurrences := analysis.References(string(text), *line-1, *char-1, *includeDeclaration)
	if len(occurrences) == 0 {
		fmt.Fprintln(stdout, "no references found")
		return 0
	}

	for _, occurrence := range occurrences {
		fmt.Fprintln(stdout, formatSymbolOccurrence(path, occurrence))
	}

	return 0
}

func formatDiagnostic(path string, diag analysis.Diagnostic) string {
	line := diag.Range.Start.Line + 1
	char := diag.Range.Start.Character + 1

	return fmt.Sprintf("%s:%d:%d: %s: %s", path, line, char, diag.Severity, diag.Message)
}

func formatCompletionItem(item analysis.CompletionItem) string {
	if item.Detail == "" {
		return fmt.Sprintf("%s\t%s", item.Label, item.Kind)
	}

	return fmt.Sprintf("%s\t%s\t%s", item.Label, item.Kind, item.Detail)
}

func formatSymbolOccurrence(path string, occurrence analysis.SymbolOccurrence) string {
	line := occurrence.Range.Start.Line + 1
	char := occurrence.Range.Start.Character + 1

	return fmt.Sprintf("%s:%d:%d: %s %s %s", path, line, char, occurrence.Role, occurrence.Kind, occurrence.Name)
}

func printUsage(w io.Writer) {
	exe := filepath.Base(os.Args[0])

	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  %s [--log-level LEVEL] [--log-file PATH] serve\n", exe)
	fmt.Fprintf(w, "  %s [--log-level LEVEL] [--log-file PATH] check <file.model>\n", exe)
	fmt.Fprintf(w, "  %s [--log-level LEVEL] [--log-file PATH] complete --line N --char N <file.model>\n", exe)
	fmt.Fprintf(w, "  %s [--log-level LEVEL] [--log-file PATH] definition --line N --char N <file.model>\n", exe)
	fmt.Fprintf(w, "  %s [--log-level LEVEL] [--log-file PATH] references --line N --char N [--include-declaration] <file.model>\n", exe)
	fmt.Fprintf(w, "  %s help\n", exe)
	fmt.Fprintf(w, "  %s version\n", exe)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "If no command is provided, stdio LSP mode starts.")
}
