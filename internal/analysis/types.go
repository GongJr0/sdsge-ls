package analysis

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
	SeverityHint    Severity = "hint"
)

type Position struct {
	Line      int
	Character int
}

type Range struct {
	Start Position
	End   Position
}

type Diagnostic struct {
	Range    Range
	Severity Severity
	Source   string
	Message  string
}

type CompletionKind string

const (
	CompletionKindUnknown  CompletionKind = "unknown"
	CompletionKindField    CompletionKind = "field"
	CompletionKindModule   CompletionKind = "module"
	CompletionKindProperty CompletionKind = "property"
	CompletionKindVariable CompletionKind = "variable"
	CompletionKindConstant CompletionKind = "constant"
	CompletionKindValue    CompletionKind = "value"
)

type CompletionItem struct {
	Label  string
	Kind   CompletionKind
	Detail string
}

type SymbolKind string

const (
	SymbolKindVariable   SymbolKind = "variable"
	SymbolKindParameter  SymbolKind = "parameter"
	SymbolKindObservable SymbolKind = "observable"
	SymbolKindShock      SymbolKind = "shock"
)

type SymbolRole string

const (
	SymbolRoleDeclaration SymbolRole = "declaration"
	SymbolRoleReference   SymbolRole = "reference"
)

type SymbolOccurrence struct {
	Name  string
	Kind  SymbolKind
	Role  SymbolRole
	Range Range
}
