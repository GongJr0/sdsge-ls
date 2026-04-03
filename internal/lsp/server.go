package lsp

import (
	"log/slog"
	"sync"
	"time"

	"github.com/GongJr0/sdsge-ls/internal/analysis"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

type Server struct {
	mu     sync.RWMutex
	docs   map[string]string
	logger *slog.Logger
}

func New(logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(ioDiscard{}, nil))
	}

	return &Server{
		docs:   make(map[string]string),
		logger: logger,
	}
}

func (s *Server) DidOpen(ctx *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
	uri := params.TextDocument.URI
	text := params.TextDocument.Text

	s.mu.Lock()
	s.docs[uri] = text
	s.mu.Unlock()

	s.logger.Debug("document opened", "uri", uri, "bytes", len(text))
	s.validateAndPublish(ctx, uri, text)
	return nil
}

func (s *Server) DidChange(ctx *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
	uri := params.TextDocument.URI

	if len(params.ContentChanges) > 0 {
		s.logger.Debug("document changed", "uri", uri, "changes", len(params.ContentChanges))

		switch change := params.ContentChanges[0].(type) {
		case protocol.TextDocumentContentChangeEventWhole:
			s.mu.Lock()
			s.docs[uri] = change.Text
			text := s.docs[uri]
			s.mu.Unlock()
			s.validateAndPublish(ctx, uri, text)
		case *protocol.TextDocumentContentChangeEventWhole:
			s.mu.Lock()
			s.docs[uri] = change.Text
			text := s.docs[uri]
			s.mu.Unlock()
			s.validateAndPublish(ctx, uri, text)
		case protocol.TextDocumentContentChangeEvent:
			s.mu.Lock()
			s.docs[uri] = change.Text
			text := s.docs[uri]
			s.mu.Unlock()
			s.validateAndPublish(ctx, uri, text)
		case *protocol.TextDocumentContentChangeEvent:
			s.mu.Lock()
			s.docs[uri] = change.Text
			text := s.docs[uri]
			s.mu.Unlock()
			s.validateAndPublish(ctx, uri, text)
		}
	}

	return nil
}

func (s *Server) DidClose(ctx *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
	uri := params.TextDocument.URI

	s.mu.Lock()
	delete(s.docs, uri)
	s.mu.Unlock()

	s.logger.Debug("document closed", "uri", uri)
	s.publishDiagnostics(ctx, uri, nil)
	return nil
}

func (s *Server) validateAndPublish(ctx *glsp.Context, uri string, text string) {
	start := time.Now()
	diags := analysis.Check(text)

	s.logger.Debug(
		"validated document",
		"uri", uri,
		"diagnostics", len(diags),
		"elapsed", time.Since(start),
	)

	s.publishDiagnostics(ctx, uri, diags)
}

func (s *Server) publishDiagnostics(ctx *glsp.Context, uri string, diags []analysis.Diagnostic) {
	if diags == nil {
		diags = []analysis.Diagnostic{}
	}

	ctx.Notify("textDocument/publishDiagnostics", protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: toProtocolDiagnostics(diags),
	})
}

func (s *Server) getDoc(uri string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[uri]
}

func (s *Server) Completion(ctx *glsp.Context, params *protocol.CompletionParams) (any, error) {
	uri := params.TextDocument.URI
	text := s.getDoc(uri)

	start := time.Now()
	items := analysis.Complete(text, int(params.Position.Line), int(params.Position.Character))

	s.logger.Debug(
		"computed completions",
		"uri", uri,
		"line", params.Position.Line,
		"character", params.Position.Character,
		"items", len(items),
		"elapsed", time.Since(start),
	)

	return toProtocolCompletionItems(items), nil
}

func (s *Server) Definition(ctx *glsp.Context, params *protocol.DefinitionParams) (any, error) {
	uri := params.TextDocument.URI
	text := s.getDoc(uri)

	start := time.Now()
	occurrence := analysis.Definition(text, int(params.Position.Line), int(params.Position.Character))

	s.logger.Debug(
		"resolved definition",
		"uri", uri,
		"line", params.Position.Line,
		"character", params.Position.Character,
		"found", occurrence != nil,
		"elapsed", time.Since(start),
	)

	if occurrence == nil {
		return nil, nil
	}

	return []protocol.Location{toProtocolLocation(uri, *occurrence)}, nil
}

func (s *Server) References(ctx *glsp.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	uri := params.TextDocument.URI
	text := s.getDoc(uri)

	start := time.Now()
	occurrences := analysis.References(
		text,
		int(params.Position.Line),
		int(params.Position.Character),
		params.Context.IncludeDeclaration,
	)

	s.logger.Debug(
		"resolved references",
		"uri", uri,
		"line", params.Position.Line,
		"character", params.Position.Character,
		"includeDeclaration", params.Context.IncludeDeclaration,
		"count", len(occurrences),
		"elapsed", time.Since(start),
	)

	return toProtocolLocations(uri, occurrences), nil
}
