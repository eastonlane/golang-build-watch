package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCmdRequiresModuleRootArgument(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "requires exactly one module-root positional argument") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRootCmdRejectsExtraPositionalArguments(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"./module", "./extra"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "requires exactly one module-root positional argument") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRootCmdExample(t *testing.T) {
	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--example", "--target", "./..."})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, needle := range []string{"GBWATCH REPORT", "example.com/demo/internal/service", "Packages : 4", "Total    : 1.15s", "Status   : success"} {
		if !strings.Contains(output, needle) {
			t.Fatalf("output missing %q:\n%s", needle, output)
		}
	}

	if !strings.Contains(stderr.String(), "example mode: showing mock build timing data") {
		t.Fatalf("stderr missing example note:\n%s", stderr.String())
	}
}

func TestRootCmdExampleJSON(t *testing.T) {
	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--example", "--json", "--target", "./..."})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v\noutput:\n%s", err, stdout.String())
	}

	if payload["status"] != "success" {
		t.Fatalf("status = %v, want success", payload["status"])
	}

	packages, ok := payload["packages"].([]any)
	if !ok || len(packages) == 0 {
		t.Fatalf("packages = %#v, want non-empty array", payload["packages"])
	}
}

func TestRootCmdExampleUsesConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "gbwatch.toml")
	if err := os.WriteFile(configPath, []byte("[settings]\noutput_format = \"json\"\nall_packages = true\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--example", "--config", configPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v\noutput:\n%s", err, stdout.String())
	}
	if !strings.Contains(stderr.String(), "example mode: showing mock build timing data") {
		t.Fatalf("stderr missing example note:\n%s", stderr.String())
	}
}

func TestRootCmdHelpDefaultEnglish(t *testing.T) {
	cmd := NewRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	for _, needle := range []string{"Usage:", "Flags:", "build target relative to module root", "help for gbwatch"} {
		if !strings.Contains(output, needle) {
			t.Fatalf("help output missing %q:\n%s", needle, output)
		}
	}
}
