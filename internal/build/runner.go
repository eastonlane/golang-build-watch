package build

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"time"

	"golang-build-watch/internal/target"
	"golang-build-watch/internal/timing"
)

type Clock func() time.Time

type Runner struct {
	Now                Clock
	IncludeAllPackages bool
	Logger             *slog.Logger
}

type Executor interface {
	Run(ctx context.Context, t target.BuildTarget) Result
}

type Result struct {
	Target string
	Timing timing.Result
	Stderr string
	Advice []string
	Err    error
}

type buildEvent struct {
	ImportPath string
	Action     string
	Output     string
}

func (r Runner) Run(ctx context.Context, t target.BuildTarget) Result {
	now := r.Now
	if now == nil {
		now = time.Now
	}

	start := now()
	cmd := exec.CommandContext(ctx, "go", "build", "-json", "-v", "-p=1", t.BuildArg)
	cmd.Dir = t.ModuleRoot

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{Err: fmt.Errorf("capture stdout: %w", err)}
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return Result{Err: fmt.Errorf("start go build: %w", err)}
	}

	packages, parseErr := parseBuildOutput(stdout, now, packageFilter(t.ModulePath, r.IncludeAllPackages), r.Logger)
	waitErr := cmd.Wait()
	end := now()

	result := Result{
		Target: t.BuildArg,
		Timing: timing.Compute(start, end, packages),
		Stderr: strings.TrimSpace(stderr.String()),
	}
	if len(packages) == 0 {
		result.Advice = append(result.Advice,
			"no package build output was received; this can happen when Go reuses the build cache",
			"try `go clean -cache` and run gbwatch again if you want fresh timings",
		)
	}

	if parseErr != nil {
		result.Err = parseErr
		return result
	}

	if waitErr != nil {
		result.Err = fmt.Errorf("go build failed: %w", waitErr)
		return result
	}

	return result
}

type ExampleRunner struct {
	IncludeAllPackages bool
}

func (r ExampleRunner) Run(ctx context.Context, t target.BuildTarget) Result {
	_ = ctx

	records := []timing.PackageRecord{
		{ImportPath: "example.com/demo/internal/config", Duration: 120 * time.Millisecond},
		{ImportPath: "example.com/demo/internal/store", Duration: 310 * time.Millisecond},
		{ImportPath: "example.com/demo/internal/service", Duration: 540 * time.Millisecond},
		{ImportPath: "example.com/demo/cmd/server", Duration: 180 * time.Millisecond},
	}

	if r.IncludeAllPackages {
		records = append(records,
			timing.PackageRecord{ImportPath: "github.com/spf13/cobra", Duration: 260 * time.Millisecond},
			timing.PackageRecord{ImportPath: "golang.org/x/sys", Duration: 90 * time.Millisecond},
		)
	}

	total := time.Duration(0)
	for _, record := range records {
		total += record.Duration
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Duration == records[j].Duration {
			return records[i].ImportPath < records[j].ImportPath
		}
		return records[i].Duration > records[j].Duration
	})

	stderr := "example mode: showing mock build timing data"
	if r.IncludeAllPackages {
		stderr += " with external dependencies included"
	}

	return Result{
		Target: ".",
		Timing: timing.Result{
			Packages:      records,
			TotalDuration: total,
		},
		Stderr: stderr,
	}
}

func parseBuildOutput(r io.Reader, now Clock, include func(string) bool, logger *slog.Logger) ([]timing.PackageRecord, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	partials := map[string]string{}
	var records []timing.PackageRecord

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var event buildEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return records, fmt.Errorf("decode build event: %w", err)
		}

		logBuildEvent(logger, now, event)

		if event.Action != "build-output" || strings.TrimSpace(event.ImportPath) == "" || event.Output == "" {
			continue
		}

		buffer := partials[event.ImportPath] + event.Output
		lines := strings.Split(buffer, "\n")
		partials[event.ImportPath] = lines[len(lines)-1]

		for _, candidate := range lines[:len(lines)-1] {
			if strings.TrimSpace(candidate) != event.ImportPath {
				continue
			}
			if !include(event.ImportPath) {
				continue
			}
			records = append(records, timing.PackageRecord{
				ImportPath: event.ImportPath,
				Completed:  now(),
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return records, fmt.Errorf("scan build output: %w", err)
	}

	return records, nil
}

func logBuildEvent(logger *slog.Logger, now Clock, event buildEvent) {
	if logger == nil {
		return
	}

	logger.Debug(
		"received go build event",
		slog.String("time", now().Format(time.RFC3339Nano)),
		slog.String("import_path", event.ImportPath),
		slog.String("action", event.Action),
		slog.String("output", strings.TrimSpace(event.Output)),
	)
}

func packageFilter(modulePath string, includeAll bool) func(string) bool {
	if includeAll {
		return func(string) bool { return true }
	}

	return func(importPath string) bool {
		return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
	}
}
