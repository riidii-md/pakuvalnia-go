package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/riidii-md/pakuvalnia-go/internal/manifest"
	"github.com/riidii-md/pakuvalnia-go/internal/releaseplan"
)

var Version = "dev"

func Run(args []string, out, stderr io.Writer) int {
	fail := func(err error) int {
		var d *manifest.Diagnostic
		if !errors.As(err, &d) {
			d = &manifest.Diagnostic{Code: "operation_failed", Field: "", Message: "operation could not be completed"}
		}
		_ = json.NewEncoder(stderr).Encode(d)
		return 1
	}
	if len(args) == 0 {
		return fail(manifest.Invalid("command", "expected validate or versions"))
	}
	if args[0] == "versions" {
		if len(args) != 1 {
			return fail(manifest.Invalid("arguments", "versions accepts no arguments"))
		}
		if err := json.NewEncoder(out).Encode(map[string]string{"pakuvalnia": Version, "go": "1.27.1", "goreleaser": "2.18.2", "syft": "1.52.0"}); err != nil {
			return fail(err)
		}
		return 0
	}
	if args[0] != "validate" {
		return fail(manifest.Invalid("command", "expected validate or versions"))
	}
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", ".", "consumer checkout")
	file := flags.String("manifest", ".pakuvalnia.yml", "checkout-relative manifest")
	tag := flags.String("tag", "", "exact vX.Y.Z tag; enables normalized plan output")
	repo := flags.String("repository", "", "owner/repository")
	commit := flags.String("commit", "", "full source commit")
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		return fail(manifest.Invalid("arguments", "invalid validate arguments"))
	}
	resolvedRoot, err := filepath.EvalSymlinks(*root)
	if err != nil {
		return fail(manifest.Invalid("root", "cannot resolve checkout"))
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return fail(manifest.Invalid("root", "cannot resolve checkout"))
	}
	if filepath.IsAbs(*file) || strings.Contains(*file, "\\") {
		return fail(manifest.Invalid("manifest", "must be a checkout-relative file"))
	}
	for _, part := range strings.Split(*file, "/") {
		if part == ".." {
			return fail(manifest.Invalid("manifest", "parent traversal is not supported"))
		}
	}
	filename, err := filepath.EvalSymlinks(filepath.Join(resolvedRoot, *file))
	if err != nil {
		return fail(manifest.Invalid("manifest", "cannot resolve manifest file"))
	}
	rel, err := filepath.Rel(resolvedRoot, filename)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fail(manifest.Invalid("manifest", "file escapes checkout"))
	}
	checkout, err := os.OpenRoot(resolvedRoot)
	if err != nil {
		return fail(manifest.Invalid("root", "cannot open checkout"))
	}
	defer checkout.Close()
	info, err := checkout.Stat(rel)
	if err != nil || !info.Mode().IsRegular() {
		return fail(manifest.Invalid("manifest", "must be a regular file inside checkout"))
	}
	f, err := checkout.Open(rel)
	if err != nil {
		return fail(manifest.Invalid("manifest", "cannot open manifest file"))
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fail(manifest.Invalid("manifest", "must be a regular file"))
	}
	m, err := manifest.LoadManifest(f)
	if err != nil {
		return fail(err)
	}
	var result any = m
	if *tag != "" || *repo != "" || *commit != "" {
		result, err = releaseplan.Resolve(context.Background(), resolvedRoot, m, releaseplan.RepositoryFacts{Repository: *repo, Commit: *commit, Tag: *tag})
		if err != nil {
			return fail(err)
		}
	} else {
		// Local validation checks filesystem/module facts without claiming release authorization.
		_, err = releaseplan.Resolve(context.Background(), resolvedRoot, m, releaseplan.RepositoryFacts{Repository: "local/validation", Commit: strings.Repeat("0", 40), Tag: "v0.0.0"})
		if err != nil {
			return fail(err)
		}
	}
	if err = json.NewEncoder(out).Encode(result); err != nil {
		return fail(err)
	}
	return 0
}
