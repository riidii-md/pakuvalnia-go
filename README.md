# pakuvalnia-go
Reusable packaging and release automation for Go tools

## Status

The first implementation slice provides strict local manifest validation and
deterministic planning for one CGO-disabled Go binary on Linux, macOS, and
Windows (amd64 and arm64). Snapshot, GitHub publication, Homebrew/Scoop, and
native package lifecycle verification are subsequent slices, not yet available.

No GoReleaser Pro subscription is needed. The planned packaging adapter uses
GoReleaser OSS; Pakuvalnia owns the manifest and release policy.

## Try the foundation

Requires Go 1.26 or newer; the engine's pinned build toolchain is Go 1.27.1.

```sh
go run ./cmd/pakuvalnia versions
go run ./cmd/pakuvalnia validate --root testdata/consumers/minimal
go test ./...
go test -race ./...
go vet ./...
```

See [manifest v1](docs/manifest-v1.md), [the editor schema](schemas/pakuvalnia-v1.schema.json),
and [next-step priorities](TODO.md). All Go packages are internal; the product
contract is the CLI and, when implemented, reusable GitHub workflows.
