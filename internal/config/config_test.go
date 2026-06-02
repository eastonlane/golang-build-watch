package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"golang-build-watch/internal/config"
)

func TestLoadUsesDefaults(t *testing.T) {
	cfg, err := config.Load(config.CLIOverrides{
		ConfigPath: filepath.Join(t.TempDir(), "missing-gbwatch.toml"),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Settings.Target != "./..." {
		t.Fatalf("Target = %q, want ./...", cfg.Settings.Target)
	}
	if cfg.Settings.OutputFormat != config.OutputFormatPretty {
		t.Fatalf("OutputFormat = %q, want pretty", cfg.Settings.OutputFormat)
	}
}

func TestLoadMergesFileAndCLI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gbwatch.toml")
	if err := os.WriteFile(path, []byte("[settings]\ntarget = \"./...\"\nall_packages = true\noutput_format = \"json\"\ndebug = true\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	target := "./cmd/server"
	debug := false
	format := config.OutputFormatTOML

	cfg, err := config.Load(config.NewCLIOverrides(
		"",
		false,
		path,
		&target,
		nil,
		&debug,
		&format,
		nil,
	))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Settings.Target != "./cmd/server" {
		t.Fatalf("Target = %q, want ./cmd/server", cfg.Settings.Target)
	}
	if !cfg.Settings.IncludeAllPackages {
		t.Fatal("expected IncludeAllPackages from file")
	}
	if cfg.Settings.Debug {
		t.Fatal("expected Debug overridden by CLI")
	}
	if cfg.Settings.OutputFormat != config.OutputFormatTOML {
		t.Fatalf("OutputFormat = %q, want toml", cfg.Settings.OutputFormat)
	}
}

func TestParseOutputFormatRejectsUnknown(t *testing.T) {
	if _, err := config.ParseOutputFormat("yaml"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOutputFormatAcceptsLegacyTable(t *testing.T) {
	format, err := config.ParseOutputFormat("table")
	if err != nil {
		t.Fatalf("ParseOutputFormat() error = %v", err)
	}
	if format != config.OutputFormatPretty {
		t.Fatalf("format = %q, want pretty", format)
	}
}
