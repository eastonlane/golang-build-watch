package target_test

import (
	"os"
	"path/filepath"
	"testing"

	"golang-build-watch/internal/target"
)

func TestResolveDefaultTarget(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root, "example.com/app")

	targetValue, err := target.Resolve(root, "")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if targetValue.BuildArg != "." {
		t.Fatalf("BuildArg = %q, want \".\"", targetValue.BuildArg)
	}
	if targetValue.ModulePath != "example.com/app" {
		t.Fatalf("ModulePath = %q, want example.com/app", targetValue.ModulePath)
	}
}

func TestResolveNestedDirectory(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root, "example.com/app")
	if err := os.MkdirAll(filepath.Join(root, "cmd", "server"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	targetValue, err := target.Resolve(root, filepath.Join("cmd", "server"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if targetValue.BuildArg != "./cmd/server" {
		t.Fatalf("BuildArg = %q, want ./cmd/server", targetValue.BuildArg)
	}
}

func TestResolvePackagePattern(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root, "example.com/app")
	if err := os.MkdirAll(filepath.Join(root, "cmd", "server"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	targetValue, err := target.Resolve(root, "./...")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if targetValue.BuildArg != "./..." {
		t.Fatalf("BuildArg = %q, want ./...", targetValue.BuildArg)
	}
}

func TestResolvePackageDirectory(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root, "example.com/app")
	if err := os.MkdirAll(filepath.Join(root, "cmd", "server"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	targetValue, err := target.Resolve(root, "cmd/server")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if targetValue.BuildArg != "./cmd/server" {
		t.Fatalf("BuildArg = %q, want ./cmd/server", targetValue.BuildArg)
	}
}

func TestResolveRejectsEscape(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root, "example.com/app")
	if _, err := target.Resolve(root, ".."); err == nil {
		t.Fatal("expected escape error")
	}
}

func TestResolveRejectsEscapingPattern(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root, "example.com/app")
	if _, err := target.Resolve(root, "../..."); err == nil {
		t.Fatal("expected escape error")
	}
}

func TestResolveMissingDirectory(t *testing.T) {
	root := t.TempDir()
	writeGoMod(t, root, "example.com/app")
	if _, err := target.Resolve(root, "cmd/server"); err == nil {
		t.Fatal("expected missing directory error")
	}
}

func TestResolveMissingGoMod(t *testing.T) {
	root := t.TempDir()
	if _, err := target.Resolve(root, "."); err == nil {
		t.Fatal("expected missing go.mod error")
	}
}

func writeGoMod(t *testing.T, root, modulePath string) {
	t.Helper()
	content := "module " + modulePath + "\n\ngo 1.26.3\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) error = %v", err)
	}
}
