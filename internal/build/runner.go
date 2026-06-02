package build

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"strconv"
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

func (r Runner) Run(ctx context.Context, t target.BuildTarget) Result {
	now := r.Now
	if now == nil {
		now = time.Now
	}

	start := now()
	eventsFile, err := os.CreateTemp("", "gbwatch-events-*.jsonl")
	if err != nil {
		return Result{Err: fmt.Errorf("create event file: %w", err)}
	}
	eventsPath := eventsFile.Name()
	eventsFile.Close()
	defer os.Remove(eventsPath)

	selfPath, err := os.Executable()
	if err != nil {
		return Result{Err: fmt.Errorf("resolve executable path: %w", err)}
	}

	toolExecCmd := strings.Join([]string{
		strconv.Quote(selfPath),
		"__toolexec",
		"--events-file",
		strconv.Quote(eventsPath),
	}, " ")

	cmd := exec.CommandContext(ctx, "go", "build", "-p=1", "-toolexec", toolExecCmd, t.BuildArg)
	cmd.Dir = t.ModuleRoot

	var stderr bytes.Buffer
	cmd.Stdout = &stderr
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return Result{Err: fmt.Errorf("start go build: %w", err)}
	}

	waitErr := cmd.Wait()
	end := now()

	durations, parseErr := readToolExecEvents(eventsPath, packageFilter(t.ModulePath, r.IncludeAllPackages))

	result := Result{
		Target: t.BuildArg,
		Timing: timing.FromDurations(durations, end.Sub(start)),
		Stderr: strings.TrimSpace(stderr.String()),
	}
	if len(durations) == 0 {
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

	targetArg := t.BuildArg
	if strings.TrimSpace(targetArg) == "" {
		targetArg = "./..."
	}

	return Result{
		Target: targetArg,
		Timing: timing.Result{
			Packages:      records,
			TotalDuration: total,
		},
		Stderr: stderr,
	}
}

func packageFilter(modulePath string, includeAll bool) func(string) bool {
	if includeAll {
		return func(string) bool { return true }
	}

	return func(importPath string) bool {
		return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
	}
}
