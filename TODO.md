# Pakuvalnia Go roadmap

This roadmap records capabilities to add after the initial managed release contract is proven. The priority groups are ordered by explicit product preference. Detailed planning may refine or split work inside a group, but must not silently reorder the groups.

The initial foundation remains a reusable workflow and bounded manifest for a conventional single-binary, CGO-disabled Go CLI, producing verified Windows, macOS, and Linux GitHub Release artifacts with Homebrew and Scoop publication.

## Priority 1: Trusted platform binaries and installers

- macOS code signing and notarization.
- Apple signing-identity and credential lifecycle support.
- Windows Authenticode signing.
- MSI packages.
- NSIS installers.

Required outcomes include isolated signing credentials, timestamping, signature verification, clean-machine installation tests, upgrade/uninstall tests, and documented key rotation and revocation recovery.

## Priority 2: Complex Go repositories and builds

- Multiple binaries from one repository.
- Monorepo-aware release selection and tagging.
- CGO cross-compilation.
- Native build dependencies and toolchains.
- Private Go module authentication.

This capability must extend the manifest and build model without weakening deterministic target resolution or exposing private-module credentials to untrusted validation runs.

## Priority 3: Hosted Linux package repositories

- APT repository publication for Debian and Ubuntu packages.
- YUM/DNF repository publication for RPM packages.
- Alpine repository publication for APK packages.

This is more than attaching package files to GitHub Releases. It requires repository indexes, signing, hosting, retention, key rotation, availability monitoring, and clean install/upgrade verification through the native package managers.

## Priority 4: Container publication

- Docker/OCI image builds.
- Multi-architecture manifests.
- Registry publication, initially targeting GHCR unless a later direction chooses otherwise.
- Image SBOMs, provenance, signing, and pull/run smoke tests.

## Priority 5: External Windows package ecosystems

- Automatic Chocolatey package publication.
- Automatic Winget manifest generation and community-repository pull-request submission.

These adapters need explicit metadata contracts, isolated credentials, external moderation status, idempotent retries, and reporting that distinguishes a successful GitHub Release from pending or failed package-index publication.
