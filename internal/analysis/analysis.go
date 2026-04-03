package analysis

import (
	"sort"

	"github.com/GongJr0/sdsge-ls/internal/validate"
	"github.com/GongJr0/sdsge-ls/internal/yamlpos"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

func Check(text string) []Diagnostic {
	parse := yamlpos.Parse(text)
	diags := fromProtocolDiagnostics(parse.Diagnostics)

	if parse.Node != nil {
		diags = append(diags, fromProtocolDiagnostics(validate.Run(parse.Node))...)
	}

	sortDiagnostics(diags)
	return diags
}

func Complete(text string, line, char int) []CompletionItem {
	items := fromProtocolCompletionItems(validate.Complete(text, line, char))
	sortCompletionItems(items)
	return items
}

func HasErrors(diags []Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity == SeverityError {
			return true
		}
	}

	return false
}

func fromProtocolDiagnostics(in []protocol.Diagnostic) []Diagnostic {
	out := make([]Diagnostic, 0, len(in))

	for _, diag := range in {
		source := ""
		if diag.Source != nil {
			source = *diag.Source
		}

		severity := SeverityError
		if diag.Severity != nil {
			severity = fromProtocolSeverity(*diag.Severity)
		}

		out = append(out, Diagnostic{
			Range: Range{
				Start: Position{
					Line:      int(diag.Range.Start.Line),
					Character: int(diag.Range.Start.Character),
				},
				End: Position{
					Line:      int(diag.Range.End.Line),
					Character: int(diag.Range.End.Character),
				},
			},
			Severity: severity,
			Source:   source,
			Message:  diag.Message,
		})
	}

	return out
}

func fromProtocolSeverity(sev protocol.DiagnosticSeverity) Severity {
	switch sev {
	case protocol.DiagnosticSeverityWarning:
		return SeverityWarning
	case protocol.DiagnosticSeverityInformation:
		return SeverityInfo
	case protocol.DiagnosticSeverityHint:
		return SeverityHint
	default:
		return SeverityError
	}
}

func fromProtocolCompletionItems(in []protocol.CompletionItem) []CompletionItem {
	out := make([]CompletionItem, 0, len(in))

	for _, item := range in {
		kind := CompletionKindUnknown
		if item.Kind != nil {
			kind = fromProtocolCompletionKind(*item.Kind)
		}

		detail := ""
		if item.Detail != nil {
			detail = *item.Detail
		}

		out = append(out, CompletionItem{
			Label:  item.Label,
			Kind:   kind,
			Detail: detail,
		})
	}

	return out
}

func fromProtocolCompletionKind(kind protocol.CompletionItemKind) CompletionKind {
	switch kind {
	case protocol.CompletionItemKindField:
		return CompletionKindField
	case protocol.CompletionItemKindModule:
		return CompletionKindModule
	case protocol.CompletionItemKindProperty:
		return CompletionKindProperty
	case protocol.CompletionItemKindVariable:
		return CompletionKindVariable
	case protocol.CompletionItemKindConstant:
		return CompletionKindConstant
	case protocol.CompletionItemKindValue:
		return CompletionKindValue
	default:
		return CompletionKindUnknown
	}
}

func sortDiagnostics(diags []Diagnostic) {
	sort.Slice(diags, func(i, j int) bool {
		left := diags[i]
		right := diags[j]

		if left.Range.Start.Line != right.Range.Start.Line {
			return left.Range.Start.Line < right.Range.Start.Line
		}
		if left.Range.Start.Character != right.Range.Start.Character {
			return left.Range.Start.Character < right.Range.Start.Character
		}
		if left.Severity != right.Severity {
			return left.Severity < right.Severity
		}
		return left.Message < right.Message
	})
}

func sortCompletionItems(items []CompletionItem) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]

		if left.Label != right.Label {
			return left.Label < right.Label
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.Detail < right.Detail
	})
}
