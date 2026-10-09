package manifest

import (
	"bytes"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const MaxBytes = 64 * 1024

type Diagnostic struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (d *Diagnostic) Error() string { return d.Code + ": " + d.Field + ": " + d.Message }
func Invalid(field, message string) error {
	return &Diagnostic{Code: "invalid_manifest", Field: field, Message: message}
}

type ManifestV1 struct {
	Schema   int              `yaml:"schema" json:"schema"`
	Project  Project          `yaml:"project" json:"project"`
	Build    Build            `yaml:"build" json:"build"`
	Version  VersionPolicy    `yaml:"version" json:"version"`
	Packages PackagePolicy    `yaml:"packages" json:"packages"`
	Channels ChannelSelection `yaml:"channels" json:"channels"`
}
type Project struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Homepage    string `yaml:"homepage" json:"homepage"`
	License     string `yaml:"license" json:"license"`
	Maintainer  string `yaml:"maintainer" json:"maintainer"`
}
type Build struct {
	Profile string `yaml:"profile" json:"profile"`
	Main    string `yaml:"main" json:"main"`
	Binary  string `yaml:"binary" json:"binary"`
}
type VersionPolicy struct {
	Variable string `yaml:"variable" json:"variable"`
	Value    string `yaml:"value" json:"value"`
	Probe    Probe  `yaml:"probe" json:"probe"`
}
type Probe struct {
	Args   []string `yaml:"args" json:"args"`
	Expect string   `yaml:"expect" json:"expect"`
}
type PackagePolicy struct {
	Formats []string `yaml:"formats" json:"formats"`
}
type ChannelSelection struct {
	Homebrew bool `yaml:"homebrew" json:"homebrew"`
	Scoop    bool `yaml:"scoop" json:"scoop"`
}

func LoadManifest(r io.Reader) (ManifestV1, error) {
	var m ManifestV1
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return m, Invalid("", "cannot read manifest")
	}
	if len(data) > MaxBytes {
		return m, Invalid("", "manifest exceeds 64 KiB")
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	var node yaml.Node
	if err = d.Decode(&node); err != nil {
		return m, Invalid("", "expected one valid YAML mapping")
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return m, Invalid("", "expected a YAML mapping")
	}
	if err = checkNode(&node, ""); err != nil {
		return m, err
	}
	var extra yaml.Node
	if d.Decode(&extra) != io.EOF {
		return m, Invalid("", "multiple YAML documents are not supported")
	}
	d = yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err = d.Decode(&m); err != nil {
		return m, Invalid("", "unknown field or invalid field type")
	}
	if err = Validate(m); err != nil {
		return m, err
	}
	return m, nil
}

func checkNode(n *yaml.Node, field string) error {
	if n.Anchor != "" || n.Kind == yaml.AliasNode {
		return Invalid("", "YAML anchors and aliases are not supported")
	}
	if n.Kind == yaml.MappingNode {
		if n.Tag != "!!map" {
			return Invalid("", "custom YAML tags are not supported")
		}
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Anchor != "" {
				return Invalid("", "YAML anchors and aliases are not supported")
			}
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value == "<<" || seen[key.Value] {
				return Invalid("", "mapping keys must be unique literal strings")
			}
			seen[key.Value] = true
			childField := key.Value
			if field != "" {
				childField = field + "." + childField
			}
			if err := checkNode(n.Content[i+1], childField); err != nil {
				return err
			}
		}
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		expected := "!!str"
		if field == "schema" {
			expected = "!!int"
		}
		if field == "channels.homebrew" || field == "channels.scoop" {
			expected = "!!bool"
		}
		if n.Tag != expected {
			return Invalid("", "invalid scalar type or unsupported YAML tag")
		}
	}
	if n.Kind == yaml.SequenceNode && n.Tag != "!!seq" {
		return Invalid("", "custom YAML tags are not supported")
	}
	for _, child := range n.Content {
		if err := checkNode(child, field); err != nil {
			return err
		}
	}
	return nil
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var symbol = regexp.MustCompile(`^(?:[A-Za-z0-9_][A-Za-z0-9_./-]*\.)[A-Za-z_][A-Za-z0-9_]*$`)

func literal(s string, max int) bool {
	return len(s) > 0 && len(s) <= max && utf8.ValidString(s) && !strings.Contains(s, "{{") && !strings.Contains(s, "}}") && strings.IndexFunc(s, unicode.IsControl) < 0
}

func Validate(m ManifestV1) error {
	if m.Schema != 1 {
		return Invalid("schema", "must equal 1")
	}
	for _, v := range []struct{ field, value string }{{"project.name", m.Project.Name}, {"build.binary", m.Build.Binary}} {
		if !identifier.MatchString(v.value) {
			return Invalid(v.field, "must be a lowercase identifier of at most 64 characters")
		}
	}
	for _, v := range []struct {
		field, value string
		max          int
	}{{"project.description", m.Project.Description, 512}, {"project.maintainer", m.Project.Maintainer, 256}, {"project.license", m.Project.License, 128}, {"project.homepage", m.Project.Homepage, 2048}} {
		if !literal(v.value, v.max) {
			return Invalid(v.field, "must be bounded literal text without controls or template delimiters")
		}
	}
	u, err := url.Parse(m.Project.Homepage)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
		return Invalid("project.homepage", "must be an absolute HTTPS URL without credentials")
	}
	if m.Build.Profile != "cli-v1" {
		return Invalid("build.profile", "must equal cli-v1")
	}
	main := m.Build.Main
	if !literal(main, 256) || strings.Contains(main, "\\") || strings.Contains(main, ":") || strings.HasPrefix(main, "/") {
		return Invalid("build.main", "must be a checkout-relative directory")
	}
	for _, part := range strings.Split(main, "/") {
		if part == ".." {
			return Invalid("build.main", "parent traversal is not supported")
		}
	}
	if main != "." && !strings.HasPrefix(main, "./") || path.Clean(main) == "" {
		return Invalid("build.main", "must be dot or start with ./")
	}
	if len(m.Version.Variable) > 256 || !symbol.MatchString(m.Version.Variable) {
		return Invalid("version.variable", "must be a Go linker symbol")
	}
	if m.Version.Value != "semantic" && m.Version.Value != "tag" {
		return Invalid("version.value", "must be semantic or tag")
	}
	if len(m.Version.Probe.Args) == 0 || len(m.Version.Probe.Args) > 16 {
		return Invalid("version.probe.args", "must contain 1 to 16 literal arguments")
	}
	for _, arg := range m.Version.Probe.Args {
		if !literal(arg, 256) {
			return Invalid("version.probe.args", "must contain bounded literal arguments")
		}
	}
	expect := m.Version.Probe.Expect
	if !literal(expect, 512) || strings.Count(expect, "{version}") != 1 || strings.ContainsAny(strings.ReplaceAll(expect, "{version}", ""), "{}") {
		return Invalid("version.probe.expect", "must contain exactly one {version} placeholder and no other braces")
	}
	seen := map[string]bool{}
	for _, format := range m.Packages.Formats {
		if seen[format] || format != "deb" && format != "rpm" && format != "apk" {
			return Invalid("packages.formats", "must be a duplicate-free subset of deb, rpm, apk")
		}
		seen[format] = true
	}
	return nil
}
