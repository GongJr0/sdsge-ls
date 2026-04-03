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

## Releases

The repository ships binaries through GitHub Actions and GoReleaser.

- CI runs on every push and pull request in [ci.yml](.github/workflows/ci.yml).
- GoReleaser publishes draft GitHub Releases with archives for:
  - `darwin/amd64`
  - `darwin/arm64`
  - `linux/amd64`
  - `linux/arm64`
  - `windows/amd64`
  - `windows/arm64`

Local snapshot build:

```bash
goreleaser release --snapshot --clean
```

The current release flow is unsigned by default. It produces release archives and
checksums, but does not code-sign macOS or Windows binaries until the required
certificates, keys, and signing steps are configured.
