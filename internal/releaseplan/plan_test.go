package releaseplan

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/riidii-md/pakuvalnia-go/internal/manifest"
)

func fixture(t testing.TB) (string, manifest.ManifestV1) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "fixture"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/fixture\n\ngo 1.26.0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m := manifest.ManifestV1{Schema: 1, Project: manifest.Project{Name: "fixture", Description: "Fixture", Homepage: "https://example.com", License: "MIT", Maintainer: "Maintainer"}, Build: manifest.Build{Profile: "cli-v1", Main: "./cmd/fixture", Binary: "fixture"}, Version: manifest.VersionPolicy{Variable: "main.version", Value: "semantic", Probe: manifest.Probe{Args: []string{"--version"}, Expect: "fixture {version}"}}}
	return root, m
}

func TestResolve(t *testing.T) {
	root, m := fixture(t)
	facts := RepositoryFacts{Repository: "riidii-md/fixture", Commit: "0123456789012345678901234567890123456789", Tag: "v1.2.3"}
	a, err := Resolve(context.Background(), root, m, facts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(context.Background(), root, m, facts)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("nondeterministic: %v", err)
	}
	if a.Version.Value != "1.2.3" || a.Version.Expected != "fixture 1.2.3" || len(a.Targets) != 6 || a.ConsumerGo != "1.26.0" {
		t.Fatalf("wrong plan: %+v", a)
	}
	wantTargets := []Target{{"linux", "amd64", "tar.gz"}, {"linux", "arm64", "tar.gz"}, {"darwin", "amd64", "tar.gz"}, {"darwin", "arm64", "tar.gz"}, {"windows", "amd64", "zip"}, {"windows", "arm64", "zip"}}
	if !reflect.DeepEqual(a.Targets, wantTargets) {
		t.Fatalf("matrix differs: %+v", a.Targets)
	}
	got, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/plan.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected ReleasePlan
	if err = json.Unmarshal(want, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, expected) {
		t.Fatalf("normalized plan changed:\n%s", got)
	}
	m.Version.Value = "tag"
	a, err = Resolve(context.Background(), root, m, facts)
	if err != nil || a.Version.Value != "v1.2.3" {
		t.Fatalf("tag policy: %+v, %v", a, err)
	}
}

func TestRejectTagAndIdentity(t *testing.T) {
	for _, tag := range []string{"v1", "1.2.3", "v1.2", "v01.2.3", "v1.2.3+metadata", "v1.2.3\n", "v1.2.3-rc.1"} {
		t.Run(tag, func(t *testing.T) {
			root, m := fixture(t)
			_, err := Resolve(context.Background(), root, m, RepositoryFacts{Repository: "riidii-md/fixture", Commit: "0123456789012345678901234567890123456789", Tag: tag})
			if err == nil {
				t.Fatal("accepted invalid exact tag")
			}
		})
	}
	root, m := fixture(t)
	for _, facts := range []RepositoryFacts{{Repository: "https://secret@host", Tag: "v1.2.3"}, {Repository: "riidii-md/fixture", Commit: "bad", Tag: "v1.2.3"}} {
		if _, err := Resolve(context.Background(), root, m, facts); err == nil {
			t.Fatal("accepted invalid identity")
		}
	}
}

func TestPathEscapeAndModule(t *testing.T) {
	root, m := fixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	m.Build.Main = "./escape"
	if _, err := Resolve(context.Background(), root, m, RepositoryFacts{Repository: "riidii-md/fixture", Commit: "0123456789012345678901234567890123456789", Tag: "v1.2.3"}); err == nil {
		t.Fatal("accepted symlink escape")
	}
	m.Build.Main = "./missing"
	if _, err := Resolve(context.Background(), root, m, RepositoryFacts{Repository: "riidii-md/fixture", Commit: "0123456789012345678901234567890123456789", Tag: "v1.2.3"}); err == nil {
		t.Fatal("accepted missing path")
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("not a module"), 0644); err != nil {
		t.Fatal(err)
	}
	m.Build.Main = "./cmd/fixture"
	if _, err := Resolve(context.Background(), root, m, RepositoryFacts{Repository: "riidii-md/fixture", Commit: "0123456789012345678901234567890123456789", Tag: "v1.2.3"}); err == nil {
		t.Fatal("accepted bad module")
	}
}

func TestModuleFileBoundary(t *testing.T) {
	for _, kind := range []string{"escape", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			root, m := fixture(t)
			if kind == "escape" {
				outside := filepath.Join(t.TempDir(), "go.mod")
				if err := os.WriteFile(outside, []byte("module example.com/outside\n\ngo 1.26.0\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(root, "go.mod"), filepath.Join(root, "go.mod.original")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, "go.mod")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/fixture\n\ngo 1.26.0\n//"+strings.Repeat("x", 1024*1024)), 0644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := Resolve(context.Background(), root, m, RepositoryFacts{Repository: "riidii-md/fixture", Commit: strings.Repeat("a", 40), Tag: "v1.2.3"})
			if err == nil {
				t.Fatal("accepted unbounded or escaped module file")
			}
		})
	}
}

func FuzzResolvePaths(f *testing.F) {
	root, m := fixture(f)
	outside := f.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		f.Fatal(err)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		f.Fatal(err)
	}
	for _, path := range []string{".", "./cmd/fixture", "../outside", "./escape", "./missing"} {
		f.Add(path)
	}
	f.Fuzz(func(t *testing.T, path string) {
		candidate := m
		candidate.Build.Main = path
		_, err := Resolve(context.Background(), root, candidate, RepositoryFacts{Repository: "riidii-md/fixture", Commit: strings.Repeat("a", 40), Tag: "v1.2.3"})
		if err != nil {
			return
		}
		resolved, err := filepath.EvalSymlinks(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if resolved != canonicalRoot && !strings.HasPrefix(resolved, canonicalRoot+string(filepath.Separator)) {
			t.Fatalf("accepted outside path")
		}
	})
}
