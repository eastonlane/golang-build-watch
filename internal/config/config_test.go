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
	format := config.OutputFormatCSV

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
	if cfg.Settings.OutputFormat != config.OutputFormatCSV {
		t.Fatalf("OutputFormat = %q, want csv", cfg.Settings.OutputFormat)
	}
}

func TestParseOutputFormatRejectsUnknown(t *testing.T) {
	for _, format := range []string{"yaml", "toml"} {
		if _, err := config.ParseOutputFormat(format); err == nil {
			t.Fatalf("ParseOutputFormat(%q) unexpectedly succeeded", format)
		}
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

func TestOutputFormatFromReportName(t *testing.T) {
	tests := []struct {
		name string
		want config.OutputFormat
		ok   bool
	}{
		{"timing.json", config.OutputFormatJSON, true},
		{"timing.CSV", config.OutputFormatCSV, true},
		{"timing.txt", config.OutputFormatPretty, true},
		{"timing", "", false},
		{"timing.log", "", false},
	}
	for _, tt := range tests {
		got, ok := config.OutputFormatFromReportName(tt.name)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("OutputFormatFromReportName(%q) = %q, %t; want %q, %t", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

func TestLoadInfersFormatFromReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gbwatch.toml")
	if err := os.WriteFile(path, []byte("[settings]\nreport_name = \"reports/timing.csv\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := config.Load(config.CLIOverrides{ConfigPath: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Settings.OutputPath != "reports/timing.csv" || cfg.Settings.OutputFormat != config.OutputFormatCSV {
		t.Fatalf("settings = %+v, want CSV report", cfg.Settings)
	}
}

func TestLoadLegacyOutputPathStillWorks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gbwatch.toml")
	if err := os.WriteFile(path, []byte("[settings]\noutput_path = \"reports/legacy.json\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := config.Load(config.CLIOverrides{ConfigPath: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Settings.OutputPath != "reports/legacy.json" || cfg.Settings.OutputFormat != config.OutputFormatJSON {
		t.Fatalf("settings = %+v, want legacy JSON report", cfg.Settings)
	}
}
