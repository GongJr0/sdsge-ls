package main

import (
	"log/slog"

	"github.com/GongJr0/sdsge-ls/internal/lsp"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

func runServer(logger *slog.Logger) int {
	ls := lsp.New(logger)
	handler := newHandler(ls)

	logger.Info("starting language server", "transport", "stdio")

	srv := server.NewServer(&handler, lsName, false)
	srv.RunStdio()
	return 0
}

func newHandler(ls *lsp.Server) protocol.Handler {
	var handler protocol.Handler

	handler = protocol.Handler{
		Initialize: func(ctx *glsp.Context, params *protocol.InitializeParams) (any, error) {
			capabilities := handler.CreateServerCapabilities()
			capabilities.TextDocumentSync = protocol.TextDocumentSyncKindFull
			capabilities.CompletionProvider = &protocol.CompletionOptions{
				TriggerCharacters: []string{":", "\n", " "},
			}

			return protocol.InitializeResult{
				Capabilities: capabilities,
				ServerInfo: &protocol.InitializeResultServerInfo{
					Name:    lsName,
					Version: &version,
				},
			}, nil
		},
		Initialized: func(ctx *glsp.Context, params *protocol.InitializedParams) error {
			return nil
		},
		Shutdown: func(ctx *glsp.Context) error {
			protocol.SetTraceValue(protocol.TraceValueOff)
			return nil
		},
		SetTrace: func(ctx *glsp.Context, params *protocol.SetTraceParams) error {
			protocol.SetTraceValue(params.Value)
			return nil
		},
		TextDocumentDefinition: ls.Definition,
		TextDocumentReferences: ls.References,
		TextDocumentDidOpen:    ls.DidOpen,
		TextDocumentDidChange:  ls.DidChange,
		TextDocumentDidClose:   ls.DidClose,
		TextDocumentCompletion: ls.Completion,
	}

	return handler
}
