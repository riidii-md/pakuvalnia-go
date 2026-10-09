# Manifest v1

The current foundation supports local validation and normalized planning only. Snapshot, publication, and package-channel commands are not implemented yet.

See [the minimal manifest](../testdata/consumers/minimal/.pakuvalnia.yml) and [editor schema](../schemas/pakuvalnia-v1.schema.json).

Run local validation:

```sh
go run ./cmd/pakuvalnia validate --root testdata/consumers/minimal
```

Add `--tag v1.2.3 --repository owner/repo --commit <full-commit>` to output the normalized six-target plan. These supplied facts do not constitute release authorization.

Schema 1 selects one `cli-v1` binary: Linux, macOS, and Windows, each on amd64 and arm64. Windows uses zip; Unix uses tar.gz. CGO is disabled by managed release policy. Consumer Go comes from go.mod, independently of the engine's build toolchain.

Required sections: schema, project, build, version. Optional packages defaults to no Linux packages; optional channels defaults to off. Formats are deb/rpm/apk; destinations and credentials are central policy, not manifest inputs.

Limits are in UTF-8 bytes: manifest 64 KiB, identifiers 64, description/expected probe output 512, maintainer/probe argument/linker symbol/main path 256, license 128, homepage 2048. Identifiers start with a lowercase letter and contain lowercase letters, digits, underscore, or hyphen. Homepage must be HTTPS without user information. Main is `.` or starts with `./`, resolves to a directory inside the checkout, and cannot traverse parents or escaped symlinks.

The consumer go.mod must be a regular file contained in the checkout and at most 1 MiB. Module reading uses Go's root-constrained filesystem API; external symlinks and special files are rejected before parsing.

The version linker symbol is package-qualified. `semantic` injects 1.2.3 for v1.2.3; `tag` injects v1.2.3. Probe args contain 1–16 literal strings, never shell commands. Expect contains exactly one `{version}` placeholder. Arguments will be passed directly to the binary when snapshot/native verification is implemented.

Unknown fields, duplicate keys, YAML anchors/aliases, multiple documents, controls, and template delimiters fail closed. Consumer metadata is literal data. Runtime validation is authoritative; the JSON schema provides basic editor checks and cannot prove filesystem containment or safe YAML structure.

Failures are stable JSON diagnostics with code, field, safe message, and optional hint. Raw YAML/tool errors, values, secrets, and environments are not echoed.
