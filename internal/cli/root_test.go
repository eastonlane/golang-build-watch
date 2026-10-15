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
	"golang-build-watch/internal/timing"
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
	for _, needle := range []string{"GBWATCH REPORT", "Target   : ./...", "example.com/demo/internal/service", "Packages : 4", "Total    : 1 second 150 milliseconds", "Status   : success"} {
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
	if strings.Contains(output, autoReportValue) || strings.Contains(output, "string[=") || strings.Contains(output, "output-path") {
		t.Fatalf("help exposes old or internal report flag:\n%s", output)
	}
	if !strings.Contains(output, "-r, --report-name string") {
		t.Fatalf("help missing report flag:\n%s", output)
	}
}

func TestRootCmdHelpTopics(t *testing.T) {
	tests := []struct {
		args    []string
		needles []string
	}{
		{[]string{"help"}, []string{"Usage:", "--report-name", "help format"}},
		{[]string{"help", "format"}, []string{"pretty, json, csv", "gbwatch help format csv"}},
		{[]string{"help", "format", "csv"}, []string{"same stem", ".json", ".csv", "import_path,duration_ms,duration_human"}},
		{[]string{"help", "csv"}, []string{"same stem", ".json", ".csv", "import_path,duration_ms,duration_human"}},
		{[]string{"help", "json"}, []string{".json report", "packages"}},
		{[]string{"help", "pretty"}, []string{"human-readable report", ".txt"}},
		{[]string{"help", "report-name"}, []string{"--report-name", "module name", "-f must agree"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			cmd := NewRootCmd()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			for _, needle := range tt.needles {
				if !strings.Contains(stdout.String(), needle) {
					t.Fatalf("help output missing %q:\n%s", needle, stdout.String())
				}
			}
		})
	}
}

func TestRootCmdHelpMatchesFlag(t *testing.T) {
	output := func(args ...string) string {
		t.Helper()
		cmd := NewRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
		return stdout.String()
	}
	if help, flag := output("help"), output("-h"); help != flag {
		t.Fatalf("help and -h differ:\n%s\n%s", help, flag)
	}
}

func TestRootCmdUnknownHelpTopicFails(t *testing.T) {
	for _, args := range [][]string{
		{"help", "not-a-topic"},
		{"help", "format", "not-a-format"},
		{"hep", "not-a-topic"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := NewRootCmd()
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("Execute() error = nil, want nonzero exit")
			}
		})
	}
}

func TestRootCmdInvalidBuildArgumentsStillFail(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"not-a-command", "another-argument"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected invalid build arguments to fail")
	}
}

func TestRootCmdVersionFlag(t *testing.T) {
	cmd := newRootCmdWithVersion("test-version")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got := strings.TrimSpace(stdout.String()); got != "gbwatch version test-version" {
		t.Fatalf("stdout = %q, want %q", got, "gbwatch version test-version")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
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
	cmd.SetArgs([]string{"--example", "--format", "json", "--report-name=" + outPath})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	expectedPath, mode := resolveOutputPathTemplate(cmd, config.AppConfig{
		Settings: config.Settings{
			OutputPath:   outPath,
			OutputFormat: config.OutputFormatJSON,
		},
	})
	if mode != outputModeExplicit {
		t.Fatalf("mode = %v, want outputModeExplicit", mode)
	}
	expectedPath = finalizeOutputPath(expectedPath, config.OutputFormatJSON, "example", build.Result{
		StartedAt: time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC),
		EndedAt:   time.Date(2026, 1, 2, 15, 4, 6, 150000000, time.UTC),
	})
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

func TestRootCmdReportSuffixMatchesExplicitFormat(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "timing")
	configPath := filepath.Join(dir, "missing-config.toml")
	run := func(args ...string) string {
		t.Helper()
		cmd := NewRootCmd()
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		cmd.SetArgs(append([]string{"--example", "--config", configPath}, args...))
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		return stderr.String()
	}

	firstPath := base + "-example-" + exampleFilenameTimestamp() + ".json"
	secondPath := base + "-example-" + exampleFilenameTimestamp() + "-2.json"
	if stderr := run("--report-name", base+".json"); !strings.Contains(stderr, "wrote output file: "+firstPath) {
		t.Fatalf("suffix-inferred run reported unexpected path: %s", stderr)
	}
	if stderr := run("--report-name", base, "-f", "json"); !strings.Contains(stderr, "wrote output file: "+secondPath) {
		t.Fatalf("explicit-format run reported unexpected path: %s", stderr)
	}
	first, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", firstPath, err)
	}
	second, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", secondPath, err)
	}
	if !bytes.Equal(first, second) || !bytes.HasPrefix(first, []byte("{\n")) {
		t.Fatalf("report content differs or is not JSON:\n%s\n%s", first, second)
	}
}

func TestRootCmdRejectsConflictingFormatAndReportSuffix(t *testing.T) {
	dir := t.TempDir()
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--example", "--config", filepath.Join(dir, "missing-config.toml"), "--report-name", filepath.Join(dir, "timing.json"), "-f", "csv"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "conflicts with output format") {
		t.Fatalf("Execute() error = %v, want extension/format conflict", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("ReadDir() = %v, %v; want no output files", entries, err)
	}
}

func TestRootCmdFileOutputKeepsEarlierReport(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "report.csv")
	args := []string{"--example", "--format", "csv", "--report-name=" + outPath}
	firstStem := filepath.Join(dir, "report-example-"+exampleFilenameTimestamp())
	secondStem := firstStem + "-2"
	firstPath := firstStem + ".csv"
	secondPath := secondStem + ".csv"

	run := func() string {
		t.Helper()
		cmd := NewRootCmd()
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		return stderr.String()
	}

	if stderr := run(); !strings.Contains(stderr, "wrote output file: "+firstPath) || !strings.Contains(stderr, "wrote output file: "+firstStem+".json") {
		t.Fatalf("first run reported unexpected path: %s", stderr)
	}
	if err := os.WriteFile(firstPath, []byte("keep earlier report"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if stderr := run(); !strings.Contains(stderr, "wrote output file: "+secondPath) || !strings.Contains(stderr, "wrote output file: "+secondStem+".json") {
		t.Fatalf("second run reported unexpected path: %s", stderr)
	}

	first, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", firstPath, err)
	}
	if string(first) != "keep earlier report" {
		t.Fatalf("first report was replaced: %q", first)
	}
	second, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", secondPath, err)
	}
	if !strings.HasPrefix(string(second), "import_path,duration_ms,duration_human\n") {
		t.Fatalf("second report has unexpected content: %q", second)
	}
	metadata, err := os.ReadFile(secondStem + ".json")
	if err != nil {
		t.Fatalf("ReadFile(metadata) error = %v", err)
	}
	if !strings.Contains(string(metadata), `"target": "./..."`) || strings.Contains(string(metadata), `"packages"`) {
		t.Fatalf("metadata JSON should contain no package rows: %s", metadata)
	}
}

func TestRootCmdCSVPairSkipsPartiallyOccupiedStem(t *testing.T) {
	dir := t.TempDir()
	firstStem := filepath.Join(dir, "report-example-"+exampleFilenameTimestamp())
	secondStem := firstStem + "-2"
	if err := os.WriteFile(firstStem+".csv", []byte("existing report"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--example", "--format", "csv", "--report-name=" + filepath.Join(dir, "report.csv")})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(firstStem + ".json"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("first JSON companion should have been removed: %v", err)
	}
	for _, ext := range []string{".json", ".csv"} {
		if _, err := os.Stat(secondStem + ext); err != nil {
			t.Fatalf("missing paired report %s: %v", secondStem+ext, err)
		}
	}
	data, err := os.ReadFile(firstStem + ".csv")
	if err != nil || string(data) != "existing report" {
		t.Fatalf("existing CSV was replaced: %q, %v", data, err)
	}
}

func TestRootCmdFileOutputAutoNaming(t *testing.T) {
	t.Chdir(t.TempDir())
	restore := stubOutputTimestampNow(t, time.Date(2026, 6, 2, 12, 34, 56, 0, time.UTC))
	defer restore()

	expectedPath := filepath.Join(".", "gbwatch-example-"+exampleFilenameTimestamp()+".json")

	cmd := NewRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--example", "--format", "json", "-r"})

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
	for _, needle := range []string{`"status": "success"`, `"target": "./..."`} {
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
	cmd.SetArgs([]string{"--example", "-r", filepath.Join(dir, "aaa"), "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	expectedPath := filepath.Join(dir, "aaa-example-"+exampleFilenameTimestamp()+".json")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", expectedPath, err)
	}
	if !strings.Contains(string(data), `"status": "success"`) {
		t.Fatalf("file missing json payload:\n%s", string(data))
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
	cmd.SetArgs([]string{"--example", "--report-name", filepath.Join(dir, "aaa"), "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	expectedPath := filepath.Join(dir, "aaa-example-"+exampleFilenameTimestamp()+".json")
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", expectedPath, err)
	}
	if !strings.Contains(string(data), `"status": "success"`) {
		t.Fatalf("file missing json payload:\n%s", string(data))
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
	cmd.SetArgs([]string{"--example", "--format", "json", "--report-name=" + outPath})

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
	cmd.SetArgs([]string{"--example", "--format", "json", "--report-name=" + outPath})

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

func TestRootCmdRejectsTOMLOutput(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--example", "--format", "toml"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "unsupported output format: toml") {
		t.Fatalf("Execute() error = %v, want unsupported TOML format", err)
	}
}

func TestRootCmdValidatesProgressPackages(t *testing.T) {
	for _, value := range []string{"0", "21"} {
		t.Run(value, func(t *testing.T) {
			cmd := NewRootCmd()
			cmd.SetArgs([]string{"--example", "--progress-packages", value})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--progress-packages must be between 1 and 20") {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
	for _, value := range []string{"1", "20"} {
		t.Run(value, func(t *testing.T) {
			cmd := NewRootCmd()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetArgs([]string{"--example", "--progress-packages", value})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
}

type slowExecutor struct {
	delay  time.Duration
	result build.Result
}

type progressExecutor struct{}

func (progressExecutor) Run(_ context.Context, _ target.BuildTarget) build.Result {
	return build.Result{}
}

func (progressExecutor) RunWithProgress(_ context.Context, _ target.BuildTarget, onProgress func(build.Progress)) build.Result {
	onProgress(build.Progress{Current: "example.com/a"})
	time.Sleep(30 * time.Millisecond)
	onProgress(build.Progress{
		Current:   "example.com/b",
		Completed: []timing.PackageRecord{{ImportPath: "example.com/a", Duration: time.Second}},
	})
	time.Sleep(30 * time.Millisecond)
	return build.Result{Target: "./..."}
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
		4,
	)

	if result.Target != "./..." {
		t.Fatalf("Target = %q, want ./...", result.Target)
	}
	if !strings.Contains(stderr.String(), "building target ./...") {
		t.Fatalf("stderr missing loading text:\n%s", stderr.String())
	}
}

func TestRunWithLoadingShowsPackageProgress(t *testing.T) {
	var stderr bytes.Buffer
	result := runWithLoading(context.Background(), progressExecutor{}, target.BuildTarget{BuildArg: "./..."}, &stderr, true, 2)
	if result.Target != "./..." || !strings.Contains(stderr.String(), "1s  example.com/a") || !strings.Contains(stderr.String(), "> example.com/b") || !strings.Contains(stderr.String(), "[ok] built target ./...") {
		t.Fatalf("result = %+v, progress = %q", result, stderr.String())
	}
}

func TestLoadingDisplayLimitsRecentPackages(t *testing.T) {
	var output bytes.Buffer
	display := newLoadingDisplay(&output, "./...", 3)
	display.progress = build.Progress{
		Current: "example.com/current",
		Completed: []timing.PackageRecord{
			{ImportPath: "example.com/old", Duration: time.Second},
			{ImportPath: "example.com/recent", Duration: 2 * time.Second},
			{ImportPath: "example.com/latest", Duration: 3 * time.Second},
		},
	}
	display.draw("|")
	got := output.String()
	if strings.Contains(got, "example.com/old") || !strings.Contains(got, "2s  example.com/recent") || !strings.Contains(got, "3s  example.com/latest") || !strings.Contains(got, "> example.com/current") {
		t.Fatalf("unexpected progress display:\n%s", got)
	}

	output.Reset()
	display = newLoadingDisplay(&output, "./...", 1)
	display.progress = build.Progress{Current: "example.com/current", Completed: []timing.PackageRecord{{ImportPath: "example.com/old", Duration: time.Second}}}
	display.draw("|")
	if strings.Contains(output.String(), "example.com/old") || !strings.Contains(output.String(), "> example.com/current") {
		t.Fatalf("unexpected single-package display:\n%s", output.String())
	}
}

func TestLoadingDisplayUpdatesOnlyChangedTerminalRows(t *testing.T) {
	var output bytes.Buffer
	display := newLoadingDisplay(&output, "./...", 2)
	display.terminal = true
	display.progress = build.Progress{Current: "example.com/a"}
	display.draw("|")
	if !strings.Contains(output.String(), "\x1b[1;36m  > example.com/a\x1b[0m") {
		t.Fatalf("active package is not emphasized: %q", output.String())
	}

	output.Reset()
	display.draw("/")
	if strings.Contains(output.String(), "\x1b[2K") || !strings.Contains(output.String(), "[/]") {
		t.Fatalf("spinner tick repainted package rows: %q", output.String())
	}

	output.Reset()
	display.progress = build.Progress{
		Current:   "example.com/b",
		Completed: []timing.PackageRecord{{ImportPath: "example.com/a", Duration: time.Second}},
	}
	display.draw("/")
	if !strings.Contains(output.String(), "\x1b[2m    1s  example.com/a\x1b[0m") || !strings.Contains(output.String(), "\x1b[1;36m  > example.com/b\x1b[0m") {
		t.Fatalf("package styles are incorrect: %q", output.String())
	}
}

func TestLoadingDisplayKeepsFinalStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		status string
	}{
		{name: "success", status: "[ok] built target ./..."},
		{name: "failure", err: errors.New("build failed"), status: "[failed] build target ./..."},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			display := newLoadingDisplay(&output, "./...", 2)
			display.terminal = true
			display.progress = build.Progress{Current: "example.com/b"}
			display.draw("|")
			output.Reset()
			display.progress = build.Progress{Completed: []timing.PackageRecord{{ImportPath: "example.com/b", Duration: time.Second}}}
			display.finish(test.err)
			if !strings.Contains(output.String(), test.status) || !strings.Contains(output.String(), "1s  example.com/b") {
				t.Fatalf("final display = %q", output.String())
			}
		})
	}
}

func TestResolveOutputPathTemplateAuto(t *testing.T) {
	cmd := NewRootCmd()

	appCfg := config.AppConfig{
		Settings: config.Settings{
			OutputFormat: config.OutputFormatJSON,
			OutputPath:   autoReportValue,
			Target:       "./cmd/server",
		},
	}

	path, mode := resolveOutputPathTemplate(cmd, appCfg)
	if mode != outputModeAuto {
		t.Fatalf("mode = %v, want outputModeAuto", mode)
	}
	if path != filepath.Join(".", "gbwatch.json") {
		t.Fatalf("path = %q", path)
	}
}

func TestNormalizeOutputPathArgsConsumesTrailingPath(t *testing.T) {
	outputPath, args := normalizeOutputPathArgs(autoReportValue, []string{"D:/repos/CID/cbase", "aaa"}, false)
	if outputPath != "aaa" {
		t.Fatalf("outputPath = %q, want aaa", outputPath)
	}
	if len(args) != 1 || args[0] != "D:/repos/CID/cbase" {
		t.Fatalf("args = %#v", args)
	}
}

func TestNormalizeOutputPathArgsKeepsModuleRootForBareFlag(t *testing.T) {
	outputPath, args := normalizeOutputPathArgs(autoReportValue, []string{"D:/repos/CID/cbase"}, false)
	if outputPath != autoReportValue || len(args) != 1 || args[0] != "D:/repos/CID/cbase" {
		t.Fatalf("outputPath = %q, args = %#v", outputPath, args)
	}
}

func TestDefaultOutputPathBaseUsesReportName(t *testing.T) {
	appCfg := config.AppConfig{Settings: config.Settings{OutputFormat: config.OutputFormatPretty}}
	if got := defaultOutputPathBase(appCfg); got != "gbwatch.txt" {
		t.Fatalf("defaultOutputPathBase() = %q", got)
	}
}

func TestResolveOutputPathTemplateExplicitKeepsExtension(t *testing.T) {
	cmd := NewRootCmd()
	appCfg := config.AppConfig{
		Settings: config.Settings{
			OutputPath:   filepath.Join("reports", "build.json"),
			OutputFormat: config.OutputFormatJSON,
		},
	}

	path, mode := resolveOutputPathTemplate(cmd, appCfg)
	if mode != outputModeExplicit {
		t.Fatalf("mode = %v, want outputModeExplicit", mode)
	}
	if path != filepath.Join("reports", "build.json") {
		t.Fatalf("path = %q", path)
	}
}

func TestResolveOutputPathTemplateExplicitOverridesWrongExtensionForStructuredFormat(t *testing.T) {
	cmd := NewRootCmd()
	appCfg := config.AppConfig{
		Settings: config.Settings{
			OutputPath:   filepath.Join("reports", "build.txt"),
			OutputFormat: config.OutputFormatCSV,
		},
	}

	path, mode := resolveOutputPathTemplate(cmd, appCfg)
	if mode != outputModeExplicit {
		t.Fatalf("mode = %v, want outputModeExplicit", mode)
	}
	if path != filepath.Join("reports", "build.csv") {
		t.Fatalf("path = %q", path)
	}
}

func TestResolveOutputPathTemplateExplicitKeepsPrettyExtensionVerbatim(t *testing.T) {
	cmd := NewRootCmd()
	appCfg := config.AppConfig{
		Settings: config.Settings{
			OutputPath:   filepath.Join("reports", "build.log"),
			OutputFormat: config.OutputFormatPretty,
		},
	}

	path, mode := resolveOutputPathTemplate(cmd, appCfg)
	if mode != outputModeExplicit {
		t.Fatalf("mode = %v, want outputModeExplicit", mode)
	}
	if path != filepath.Join("reports", "build.log") {
		t.Fatalf("path = %q", path)
	}
}

func TestFinalizeOutputPathUsesModuleAndBuildStart(t *testing.T) {
	result := build.Result{
		StartedAt: time.Date(2026, 6, 3, 8, 4, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 6, 3, 8, 4, 3, 0, time.UTC),
	}

	path := finalizeOutputPath(filepath.Join("reports", "build.csv"), config.OutputFormatCSV, "my-module", result)
	want := filepath.Join("reports", "build-my-module-"+result.StartedAt.In(time.Local).Format("20060102-150405-0700")+".csv")
	if path != want {
		t.Fatalf("path = %q", path)
	}
}

func TestFinalizeOutputPathUsesLocalOffset(t *testing.T) {
	startedAt := time.Date(2026, 10, 14, 7, 9, 59, 0, time.UTC)
	result := build.Result{StartedAt: startedAt}
	path := finalizeOutputPath("report.json", config.OutputFormatJSON, "module", result)
	want := "report-module-" + startedAt.In(time.Local).Format("20060102-150405-0700") + ".json"
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func exampleFilenameTimestamp() string {
	return time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC).In(time.Local).Format("20060102-150405-0700")
}

func stubOutputTimestampNow(t *testing.T, ts time.Time) func() {
	t.Helper()
	previous := outputTimestampNow
	outputTimestampNow = func() time.Time { return ts }
	return func() {
		outputTimestampNow = previous
	}
}
