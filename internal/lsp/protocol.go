package lsp

import (
	"github.com/GongJr0/sdsge-ls/internal/analysis"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

func toProtocolDiagnostics(diags []analysis.Diagnostic) []protocol.Diagnostic {
	out := make([]protocol.Diagnostic, 0, len(diags))

	for _, diag := range diags {
		severity := toProtocolSeverity(diag.Severity)

		item := protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      uint32(diag.Range.Start.Line),
					Character: uint32(diag.Range.Start.Character),
				},
				End: protocol.Position{
					Line:      uint32(diag.Range.End.Line),
					Character: uint32(diag.Range.End.Character),
				},
			},
			Severity: &severity,
			Message:  diag.Message,
		}

		if diag.Source != "" {
			item.Source = strPtr(diag.Source)
		}

		out = append(out, item)
	}

	return out
}

func toProtocolSeverity(severity analysis.Severity) protocol.DiagnosticSeverity {
	switch severity {
	case analysis.SeverityWarning:
		return protocol.DiagnosticSeverityWarning
	case analysis.SeverityInfo:
		return protocol.DiagnosticSeverityInformation
	case analysis.SeverityHint:
		return protocol.DiagnosticSeverityHint
	default:
		return protocol.DiagnosticSeverityError
	}
}

func toProtocolCompletionItems(items []analysis.CompletionItem) []protocol.CompletionItem {
	out := make([]protocol.CompletionItem, 0, len(items))

	for _, item := range items {
		kind := toProtocolCompletionKind(item.Kind)
		entry := protocol.CompletionItem{
			Label: item.Label,
			Kind:  &kind,
		}

		if item.Detail != "" {
			entry.Detail = strPtr(item.Detail)
		}

		out = append(out, entry)
	}

	return out
}

func toProtocolLocations(uri string, occurrences []analysis.SymbolOccurrence) []protocol.Location {
	out := make([]protocol.Location, 0, len(occurrences))
	for _, occurrence := range occurrences {
		out = append(out, toProtocolLocation(uri, occurrence))
	}
	return out
}

func toProtocolLocation(uri string, occurrence analysis.SymbolOccurrence) protocol.Location {
	return protocol.Location{
		URI: protocol.DocumentUri(uri),
		Range: protocol.Range{
			Start: protocol.Position{
				Line:      uint32(occurrence.Range.Start.Line),
				Character: uint32(occurrence.Range.Start.Character),
			},
			End: protocol.Position{
				Line:      uint32(occurrence.Range.End.Line),
				Character: uint32(occurrence.Range.End.Character),
			},
		},
	}
}

func toProtocolCompletionKind(kind analysis.CompletionKind) protocol.CompletionItemKind {
	switch kind {
	case analysis.CompletionKindField:
		return protocol.CompletionItemKindField
	case analysis.CompletionKindModule:
		return protocol.CompletionItemKindModule
	case analysis.CompletionKindProperty:
		return protocol.CompletionItemKindProperty
	case analysis.CompletionKindVariable:
		return protocol.CompletionItemKindVariable
	case analysis.CompletionKindConstant:
		return protocol.CompletionItemKindConstant
	case analysis.CompletionKindValue:
		return protocol.CompletionItemKindValue
	default:
		return protocol.CompletionItemKindText
	}
}

func strPtr(s string) *string {
	return &s
}
