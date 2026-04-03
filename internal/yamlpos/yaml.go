package yamlpos

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
	"go.yaml.in/yaml/v3"
)

var lineRE = regexp.MustCompile(`line (\d+)`)

type ParseResult struct {
	Node        *yaml.Node
	Diagnostics []protocol.Diagnostic
	ErrorLine   int
}

func Parse(text string) ParseResult {
	var root yaml.Node
	err := yaml.Unmarshal([]byte(text), &root)
	if err == nil {
		return ParseResult{Node: &root}
	}

	line := parseErrorLine(err)
	return ParseResult{
		Node: nil,
		Diagnostics: []protocol.Diagnostic{
			parseErrorDiagnostic(err),
		},
		ErrorLine: line,
	}
}

func ParseBestEffort(text string) *yaml.Node {
	parse := Parse(text)
	if parse.Node != nil {
		return parse.Node
	}
	if parse.ErrorLine <= 0 {
		return nil
	}

	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")

	for cut := min(parse.ErrorLine, len(lines)); cut > 0; cut-- {
		candidate := strings.Join(lines[:cut], "\n")
		if strings.TrimSpace(candidate) == "" {
			continue
		}

		var root yaml.Node
		if err := yaml.Unmarshal([]byte(candidate), &root); err == nil {
			return &root
		}
	}

	return nil
}

func parseErrorDiagnostic(err error) protocol.Diagnostic {
	severity := protocol.DiagnosticSeverityError
	source := "sdsge-ls"

	msg := err.Error()
	line := parseErrorLine(err)

	return protocol.Diagnostic{
		Range: protocol.Range{
			Start: protocol.Position{Line: uint32(line), Character: 0},
			End:   protocol.Position{Line: uint32(line), Character: 1},
		},
		Severity: &severity,
		Source:   &source,
		Message:  fmt.Sprintf("YAML parse error: %s", msg),
	}
}

func parseErrorLine(err error) int {
	if err == nil {
		return 0
	}

	msg := err.Error()
	m := lineRE.FindStringSubmatch(msg)
	if len(m) == 2 {
		if n, convErr := strconv.Atoi(m[1]); convErr == nil && n > 0 {
			return n - 1
		}
	}

	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
