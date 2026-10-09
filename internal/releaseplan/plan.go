package releaseplan

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/riidii-md/pakuvalnia-go/internal/manifest"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

type RepositoryFacts struct {
	Repository string
	Commit     string
	Tag        string
}
type SourceIdentity struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Tag        string `json:"tag"`
}
type Version struct {
	Tag      string   `json:"tag"`
	Value    string   `json:"value"`
	Variable string   `json:"variable"`
	Args     []string `json:"args"`
	Expected string   `json:"expected"`
}
type Target struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Format string `json:"format"`
}
type Toolchain struct {
	Go         string `json:"go"`
	GoReleaser string `json:"goreleaser"`
	Syft       string `json:"syft"`
}
type ReleasePlan struct {
	Project    manifest.Project `json:"project"`
	Build      manifest.Build   `json:"build"`
	Source     SourceIdentity   `json:"source"`
	Version    Version          `json:"version"`
	Targets    []Target         `json:"targets"`
	Packages   []string         `json:"packages"`
	Channels   []string         `json:"channels"`
	Toolchain  Toolchain        `json:"toolchain"`
	ConsumerGo string           `json:"consumer_go"`
}

var exactTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var repository = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,99}/[A-Za-z0-9][A-Za-z0-9_.-]{0,99}$`)
var commit = regexp.MustCompile(`^[0-9a-f]{40}$`)

func Resolve(ctx context.Context, root string, m manifest.ManifestV1, facts RepositoryFacts) (ReleasePlan, error) {
	var p ReleasePlan
	if err := ctx.Err(); err != nil {
		return p, err
	}
	if err := manifest.Validate(m); err != nil {
		return p, err
	}
	if !exactTag.MatchString(facts.Tag) || !semver.IsValid(facts.Tag) {
		return p, manifest.Invalid("source.tag", "must be an exact vX.Y.Z tag")
	}
	if !repository.MatchString(facts.Repository) || !commit.MatchString(facts.Commit) {
		return p, manifest.Invalid("source", "repository and full lowercase Git commit are required")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return p, manifest.Invalid("build.main", "cannot resolve checkout")
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return p, manifest.Invalid("build.main", "cannot resolve checkout")
	}
	main, err := filepath.EvalSymlinks(filepath.Join(resolvedRoot, filepath.FromSlash(m.Build.Main)))
	if err != nil {
		return p, manifest.Invalid("build.main", "directory does not exist")
	}
	rel, err := filepath.Rel(resolvedRoot, main)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return p, manifest.Invalid("build.main", "directory escapes checkout")
	}
	info, err := os.Stat(main)
	if err != nil || !info.IsDir() {
		return p, manifest.Invalid("build.main", "must resolve to a directory")
	}
	checkout, err := os.OpenRoot(resolvedRoot)
	if err != nil {
		return p, manifest.Invalid("go.mod", "cannot open checkout")
	}
	defer checkout.Close()
	moduleInfo, err := checkout.Stat("go.mod")
	if err != nil || !moduleInfo.Mode().IsRegular() {
		return p, manifest.Invalid("go.mod", "must be a regular file inside checkout")
	}
	moduleFile, err := checkout.Open("go.mod")
	if err != nil {
		return p, manifest.Invalid("go.mod", "cannot read module file inside checkout")
	}
	defer moduleFile.Close()
	moduleInfo, err = moduleFile.Stat()
	if err != nil || !moduleInfo.Mode().IsRegular() {
		return p, manifest.Invalid("go.mod", "must be a regular file")
	}
	const maxModuleBytes = 1024 * 1024
	moduleBytes, err := io.ReadAll(io.LimitReader(moduleFile, maxModuleBytes+1))
	if err != nil {
		return p, manifest.Invalid("go.mod", "cannot read module file")
	}
	if len(moduleBytes) > maxModuleBytes {
		return p, manifest.Invalid("go.mod", "module file exceeds 1 MiB")
	}
	module, err := modfile.Parse("go.mod", moduleBytes, nil)
	if err != nil || module.Module == nil || module.Go == nil {
		return p, manifest.Invalid("go.mod", "valid module and Go directives are required")
	}
	value := facts.Tag
	if m.Version.Value == "semantic" {
		value = strings.TrimPrefix(value, "v")
	}
	p = ReleasePlan{Project: m.Project, Build: m.Build, Source: SourceIdentity{facts.Repository, facts.Commit, facts.Tag}, Version: Version{facts.Tag, value, m.Version.Variable, append([]string{}, m.Version.Probe.Args...), strings.ReplaceAll(m.Version.Probe.Expect, "{version}", value)}, Packages: append([]string{}, m.Packages.Formats...), Channels: []string{}, Toolchain: Toolchain{"1.27.1", "2.18.2", "1.52.0"}, ConsumerGo: module.Go.Version}
	sort.Strings(p.Packages)
	if m.Channels.Homebrew {
		p.Channels = append(p.Channels, "homebrew")
	}
	if m.Channels.Scoop {
		p.Channels = append(p.Channels, "scoop")
	}
	for _, osName := range []string{"linux", "darwin", "windows"} {
		for _, arch := range []string{"amd64", "arm64"} {
			format := "tar.gz"
			if osName == "windows" {
				format = "zip"
			}
			p.Targets = append(p.Targets, Target{osName, arch, format})
		}
	}
	return p, nil
}
