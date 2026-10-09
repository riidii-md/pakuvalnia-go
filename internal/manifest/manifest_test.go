package manifest

import (
	"errors"
	"strings"
	"testing"
)

const valid = `schema: 1
project:
  name: fixture
  description: A fixture CLI
  homepage: https://github.com/riidii-md/fixture
  license: MIT
  maintainer: Fixture Maintainer
build:
  profile: cli-v1
  main: ./cmd/fixture
  binary: fixture
version:
  variable: main.version
  value: semantic
  probe:
    args: ["--version"]
    expect: "fixture {version}"
packages:
  formats: [deb, rpm, apk]
channels:
  homebrew: true
  scoop: true
`

func TestLoad(t *testing.T) {
	m, err := LoadManifest(strings.NewReader(valid))
	if err != nil {
		t.Fatal(err)
	}
	if m.Build.Binary != "fixture" || !m.Channels.Scoop || len(m.Packages.Formats) != 3 {
		t.Fatalf("unexpected manifest: %+v", m)
	}
}

func TestRejectInvalid(t *testing.T) {
	cases := map[string]string{
		"root key anchor":          strings.Replace(valid, "schema: 1", "&schema_key schema: 1", 1),
		"nested key anchor":        strings.Replace(valid, "  name: fixture", "  &name_key name: fixture", 1),
		"numeric metadata":         strings.Replace(valid, "  license: MIT", "  license: 123", 1),
		"null channel":             strings.Replace(valid, "  scoop: true", "  scoop: null", 1),
		"custom tag":               strings.Replace(valid, "  license: MIT", "  license: !custom MIT", 1),
		"unknown":                  valid + "unexpected: secret\n",
		"duplicate":                strings.Replace(valid, "schema: 1", "schema: 1\nschema: 1", 1),
		"nested duplicate":         strings.Replace(valid, "  binary: fixture", "  binary: fixture\n  binary: other", 1),
		"alias":                    strings.Replace(valid, "  name: fixture", "  name: &name fixture", 1),
		"multiple documents":       valid + "---\nschema: 1\n",
		"empty document":           valid + "---\n",
		"schema":                   strings.Replace(valid, "schema: 1", "schema: 2", 1),
		"profile":                  strings.Replace(valid, "cli-v1", "anything", 1),
		"traversal":                strings.Replace(valid, "./cmd/fixture", "../fixture", 1),
		"absolute":                 strings.Replace(valid, "./cmd/fixture", "/tmp/fixture", 1),
		"windows path":             strings.Replace(valid, "./cmd/fixture", `C:\fixture`, 1),
		"identifier":               strings.Replace(valid, "  binary: fixture", "  binary: ../fixture", 1),
		"URL":                      strings.Replace(valid, "https://github.com", "http://github.com", 1),
		"URL credentials":          strings.Replace(valid, "https://github.com", "https://secret@github.com", 1),
		"metadata template":        strings.Replace(valid, "A fixture CLI", "'{{ .Env.TOKEN }}'", 1),
		"metadata close delimiter": strings.Replace(valid, "Fixture Maintainer", "'literal }}'", 1),
		"symbol":                   strings.Replace(valid, "main.version", "main.version -X bad", 1),
		"version value":            strings.Replace(valid, "value: semantic", "value: latest", 1),
		"probe placeholder":        strings.Replace(valid, "fixture {version}", "fixture {secret}", 1),
		"probe control":            strings.Replace(valid, `args: ["--version"]`, `args: ["--version\nsecret"]`, 1),
		"package duplicate":        strings.Replace(valid, "[deb, rpm, apk]", "[deb, deb]", 1),
		"package unknown":          strings.Replace(valid, "[deb, rpm, apk]", "[msi]", 1),
		"missing":                  "schema: 1\n",
		"empty":                    "",
		"malformed":                "schema: [\nsecret",
		"too large":                strings.Repeat("x", MaxBytes+1),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadManifest(strings.NewReader(input))
			var diagnostic *Diagnostic
			if !errors.As(err, &diagnostic) || diagnostic.Code == "" {
				t.Fatalf("expected coded diagnostic, got %v", err)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "TOKEN") {
				t.Fatalf("unsafe diagnostic: %v", err)
			}
		})
	}
}

func TestTagPolicyAndRootMain(t *testing.T) {
	input := strings.ReplaceAll(valid, "value: semantic", "value: tag")
	input = strings.ReplaceAll(input, "./cmd/fixture", ".")
	if _, err := LoadManifest(strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

func FuzzLoadManifest(f *testing.F) {
	f.Add(valid)
	f.Add("schema: 1\n")
	f.Add("a: &a [*a]\n")
	f.Fuzz(func(t *testing.T, input string) { _, _ = LoadManifest(strings.NewReader(input)) })
}
