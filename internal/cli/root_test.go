package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"golang-build-watch/internal/build"
	"golang-build-watch/internal/config"
	"golang-build-watch/internal/target"
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
	for _, needle := range []string{"GBWATCH REPORT", "Target   : ./...", "example.com/demo/internal/service", "Packages : 4", "Total    : 1.15s", "Status   : success"} {
		if !strings.Contains(output, needle) {
			t.Fatalf("output missing %q:\n%s", needle, output)
		}
	}

	if !strings.Contains(stderr.String(), "example mode: showing mock build timing data") {
		t.Fatalf("stderr missing example note:\n%s", stderr.String())
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

	output := stdout.String()
	for _, needle := range []string{
		"\"status\": \"success\"",
		"\"target\": \"./...\"",
		"\"import_path\": \"github.com/spf13/cobra\"",
	} {
		if !strings.Contains(output, needle) {
			t.Fatalf("output missing %q:\n%s", needle, output)
		}
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

func TestRootCmdFileOutputJSON(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.json")
	restore := stubOutputTimestampNow(t, time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC))
	defer restore()

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--example", "--format", "json", "--output-path=" + outPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	expectedPath, mode := resolveOutputPath(cmd, config.AppConfig{
		Settings: config.Settings{
			OutputPath:   outPath,
			OutputFormat: config.OutputFormatJSON,
		},
	}, target.BuildTarget{})
	if mode != outputModeExplicit {
		t.Fatalf("mode = %v, want outputModeExplicit", mode)
	}
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when output-mode=file", stdout.String())
	}
	if !strings.Contains(string(data), "\"status\": \"success\"") {
		t.Fatalf("file missing json payload:\n%s", string(data))
	}
	if !strings.Contains(stderr.String(), "wrote output file: "+expectedPath) {
		t.Fatalf("stderr missing output path note:\n%s", stderr.String())
	}
}

func TestRootCmdFileOutputAutoNaming(t *testing.T) {
	restore := stubOutputTimestampNow(t, time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC))
	defer restore()

	expectedPath := filepath.Join(".", "gbwatch-example-all-packages-20260602-123456.toml")
	_ = os.Remove(expectedPath)
	defer os.Remove(expectedPath)

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--example", "--format", "toml", "-o"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", expectedPath, err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when output-mode=file", stdout.String())
	}
	for _, needle := range []string{"Status = \"success\"", "Target = \"./...\""} {
		if !strings.Contains(string(data), needle) {
			t.Fatalf("file missing %q:\n%s", needle, string(data))
		}
	}
	if !strings.Contains(stderr.String(), "wrote output file: "+expectedPath) {
		t.Fatalf("stderr missing output path note:\n%s", stderr.String())
	}
}

func TestRootCmdFileOutputShortFlagConsumesTrailingPath(t *testing.T) {
	dir := t.TempDir()
	restore := stubOutputTimestampNow(t, time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC))
	defer restore()

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--example", "-o", filepath.Join(dir, "aaa"), "--format", "toml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	expectedPath := filepath.Join(dir, "aaa-20260602-123456.toml")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", expectedPath, err)
	}
	if !strings.Contains(string(data), "Status = \"success\"") {
		t.Fatalf("file missing toml payload:\n%s", string(data))
	}
}

func TestRootCmdFileOutputLongFlagConsumesTrailingPath(t *testing.T) {
	dir := t.TempDir()
	restore := stubOutputTimestampNow(t, time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC))
	defer restore()

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--example", "--output-path", filepath.Join(dir, "aaa"), "--format", "toml"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	expectedPath := filepath.Join(dir, "aaa-20260602-123456.toml")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", expectedPath, err)
	}
	if !strings.Contains(string(data), "Status = \"success\"") {
		t.Fatalf("file missing toml payload:\n%s", string(data))
	}
}

func TestRootCmdFileOutputFallsBackToPrettyForEmptyResults(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.json")

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		result := build.Result{
			Target: "./...",
		}
		if shouldWriteOutputFile(result) {
			t.Fatal("shouldWriteOutputFile() = true, want false")
		}
		if err := printPrettyConsoleResult(cmd, result); err != nil {
			return err
		}
		return result.Err
	}
	cmd.SetArgs([]string{"--example", "--format", "json", "--output-path=" + outPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, err := os.Stat(outPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(%q) error = %v, want not exists", outPath, err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty in file mode fallback", stdout.String())
	}
	for _, needle := range []string{"GBWATCH REPORT", "Status   : success", "Packages : 0", "No package timing rows were captured."} {
		if !strings.Contains(stderr.String(), needle) {
			t.Fatalf("stderr missing %q:\n%s", needle, stderr.String())
		}
	}
	if strings.Contains(stderr.String(), "wrote output file:") {
		t.Fatalf("stderr unexpectedly reported file write:\n%s", stderr.String())
	}
}

func TestRootCmdFileOutputFallsBackToPrettyForFailures(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.json")

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	expectedErr := errors.New("go build failed: exit status 1")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		result := build.Result{
			Target: "./...",
			Err:    expectedErr,
		}
		if shouldWriteOutputFile(result) {
			t.Fatal("shouldWriteOutputFile() = true, want false")
		}
		if err := printPrettyConsoleResult(cmd, result); err != nil {
			return err
		}
		return result.Err
	}
	cmd.SetArgs([]string{"--example", "--format", "json", "--output-path=" + outPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != expectedErr.Error() {
		t.Fatalf("Execute() error = %v, want %v", err, expectedErr)
	}

	if _, err := os.Stat(outPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(%q) error = %v, want not exists", outPath, err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty in file mode fallback", stdout.String())
	}
	for _, needle := range []string{"GBWATCH REPORT", "Status   : failed", "Build    : failed; timings below may be partial"} {
		if !strings.Contains(stderr.String(), needle) {
			t.Fatalf("stderr missing %q:\n%s", needle, stderr.String())
		}
	}
	if strings.Contains(stderr.String(), "wrote output file:") {
		t.Fatalf("stderr unexpectedly reported file write:\n%s", stderr.String())
	}
}

func TestRootCmdRejectsUnknownJSONFlag(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--example", "--json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
}

type slowExecutor struct {
	delay  time.Duration
	result build.Result
}

func (e slowExecutor) Run(_ context.Context, _ target.BuildTarget) build.Result {
	time.Sleep(e.delay)
	return e.result
}

func TestRunWithLoadingShowsImmediateMessage(t *testing.T) {
	var stderr bytes.Buffer
	result := runWithLoading(
		context.Background(),
		slowExecutor{
			delay:  150 * time.Millisecond,
			result: build.Result{Target: "./..."},
		},
		target.BuildTarget{BuildArg: "./..."},
		&stderr,
		true,
	)

	if result.Target != "./..." {
		t.Fatalf("Target = %q, want ./...", result.Target)
	}
	if !strings.Contains(stderr.String(), "building target ./...") {
		t.Fatalf("stderr missing loading text:\n%s", stderr.String())
	}
}

func TestResolveOutputPathAuto(t *testing.T) {
	restore := stubOutputTimestampNow(t, time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC))
	defer restore()

	cmd := NewRootCmd()

	appCfg := config.AppConfig{
		Settings: config.Settings{
			OutputFormat: config.OutputFormatJSON,
			OutputPath:   autoOutputPathValue,
			Target:       "./cmd/server",
		},
	}

	path, mode := resolveOutputPath(cmd, appCfg, target.BuildTarget{})
	if mode != outputModeAuto {
		t.Fatalf("mode = %v, want outputModeAuto", mode)
	}
	if path != filepath.Join(".", "gbwatch-example-cmd-server-20260602-123456.json") {
		t.Fatalf("path = %q", path)
	}
}

func TestNormalizeOutputPathArgsConsumesTrailingPath(t *testing.T) {
	outputPath, args := normalizeOutputPathArgs(autoOutputPathValue, []string{"D:/repos/CID/cbase", "aaa"})
	if outputPath != "aaa" {
		t.Fatalf("outputPath = %q, want aaa", outputPath)
	}
	if len(args) != 1 || args[0] != "D:/repos/CID/cbase" {
		t.Fatalf("args = %#v", args)
	}
}

func TestFormatOutputTargetSegment(t *testing.T) {
	tests := map[string]string{
		"./...":        "all-packages",
		"...":          "all-packages",
		".":            "root-package",
		"./":           "root-package",
		"./cmd/server": "cmd-server",
		"cmd/server":   "cmd-server",
		"./cmd.v2/api": "cmd-v2-api",
		"weird target": "weird-target",
	}

	for input, want := range tests {
		if got := formatOutputTargetSegment(input); got != want {
			t.Fatalf("formatOutputTargetSegment(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolveOutputPathExplicitAddsTimestamp(t *testing.T) {
	restore := stubOutputTimestampNow(t, time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC))
	defer restore()

	cmd := NewRootCmd()
	appCfg := config.AppConfig{
		Settings: config.Settings{
			OutputPath:   filepath.Join("reports", "build.json"),
			OutputFormat: config.OutputFormatJSON,
		},
	}

	path, mode := resolveOutputPath(cmd, appCfg, target.BuildTarget{})
	if mode != outputModeExplicit {
		t.Fatalf("mode = %v, want outputModeExplicit", mode)
	}
	if path != filepath.Join("reports", "build-20260602-123456.json") {
		t.Fatalf("path = %q", path)
	}
}

func TestResolveOutputPathExplicitOverridesWrongExtensionForStructuredFormat(t *testing.T) {
	restore := stubOutputTimestampNow(t, time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC))
	defer restore()

	cmd := NewRootCmd()
	appCfg := config.AppConfig{
		Settings: config.Settings{
			OutputPath:   filepath.Join("reports", "build.txt"),
			OutputFormat: config.OutputFormatCSV,
		},
	}

	path, mode := resolveOutputPath(cmd, appCfg, target.BuildTarget{})
	if mode != outputModeExplicit {
		t.Fatalf("mode = %v, want outputModeExplicit", mode)
	}
	if path != filepath.Join("reports", "build-20260602-123456.csv") {
		t.Fatalf("path = %q", path)
	}
}

func TestResolveOutputPathExplicitKeepsPrettyExtensionVerbatim(t *testing.T) {
	restore := stubOutputTimestampNow(t, time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC))
	defer restore()

	cmd := NewRootCmd()
	appCfg := config.AppConfig{
		Settings: config.Settings{
			OutputPath:   filepath.Join("reports", "build.log"),
			OutputFormat: config.OutputFormatPretty,
		},
	}

	path, mode := resolveOutputPath(cmd, appCfg, target.BuildTarget{})
	if mode != outputModeExplicit {
		t.Fatalf("mode = %v, want outputModeExplicit", mode)
	}
	if path != filepath.Join("reports", "build-20260602-123456.log") {
		t.Fatalf("path = %q", path)
	}
}

func stubOutputTimestampNow(t *testing.T, ts time.Time) func() {
	t.Helper()
	previous := outputTimestampNow
	outputTimestampNow = func() time.Time { return ts }
	return func() {
		outputTimestampNow = previous
	}
}
