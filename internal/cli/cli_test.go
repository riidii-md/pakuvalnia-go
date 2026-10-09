package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersions(t *testing.T) {
	var out, stderr bytes.Buffer
	if code := Run([]string{"versions"}, &out, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["goreleaser"] != "2.18.2" || result["go"] != "1.27.1" {
		t.Fatalf("wrong versions: %v", result)
	}
}

func TestValidateFixture(t *testing.T) {
	root, err := filepath.Abs("../../testdata/consumers/minimal")
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := Run([]string{"validate", "--root", root}, &out, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(out.String(), `"binary":"fixture"`) {
		t.Fatal(out.String())
	}
	out.Reset()
	args := []string{"validate", "--root", root, "--tag", "v1.2.3", "--repository", "riidii-md/fixture", "--commit", strings.Repeat("a", 40)}
	if code := Run(args, &out, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(out.String(), `"targets"`) || !strings.Contains(out.String(), `fixture 1.2.3`) {
		t.Fatal(out.String())
	}
}

func TestSafeErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".pakuvalnia.yml"), []byte("unknown: secret"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"unknown"}, {"versions", "--bad=secret"}, {"validate", "--bad=secret"}, {"validate", "--root", root}, {"validate", "--root", root, "--manifest", "../secret"}, {"validate", "--root", root, "--manifest", "missing"}, {"validate", "--root", "/does/not/exist"}} {
		var out, stderr bytes.Buffer
		if Run(args, &out, &stderr) == 0 {
			t.Fatalf("accepted %v", args)
		}
		if strings.Contains(stderr.String(), "secret") {
			t.Fatal(stderr.String())
		}
		var diagnostic map[string]any
		if json.Unmarshal(stderr.Bytes(), &diagnostic) != nil || diagnostic["code"] == nil {
			t.Fatal(stderr.String())
		}
	}
}

func TestManifestFileContainment(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "manifest.yml")
	if err := os.WriteFile(outside, []byte("schema: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".pakuvalnia.yml")); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if Run([]string{"validate", "--root", root}, &out, &stderr) == 0 {
		t.Fatal("accepted external manifest")
	}
	if !strings.Contains(stderr.String(), `"field":"manifest"`) {
		t.Fatal(stderr.String())
	}
	out.Reset()
	stderr.Reset()
	if Run([]string{"validate", "--root", root, "--manifest", "."}, &out, &stderr) == 0 {
		t.Fatal("accepted directory manifest")
	}
}
