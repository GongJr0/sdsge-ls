# sdsge-ls
Language Server for SymbolicDSGE model configuration files

## Development workflow

The binary can run either as a stdio language server or as a small CLI for
testing parser, validation, and completion behavior.

```bash
go run ./cmd/sdsge-ls check test_configs/test.model
go run ./cmd/sdsge-ls complete --line 104 --char 3 test_configs/test.model
go run ./cmd/sdsge-ls definition --line 22 --char 14 test_configs/test.model
go run ./cmd/sdsge-ls references --line 35 --char 15 --include-declaration test_configs/test.model
go run ./cmd/sdsge-ls --log-level debug --log-file /tmp/sdsge-ls.log
```

- No command starts the stdio LSP server.
- `check` prints plain text diagnostics.
- `complete` prints plain text completion items using 1-based `--line` and `--char`.
- `definition` prints the declaration location for the symbol under the cursor.
- `references` prints the references for the symbol under the cursor.
- `--log-level` accepts `debug`, `info`, `warn`, `error`, or `off`.
- `--log-file` writes logs to a file instead of `stderr`.
