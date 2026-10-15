package build

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

const MaxProgressPackages = 20

type Runner struct {
	Now                Clock
	IncludeAllPackages bool
	Logger             *slog.Logger
}

type Executor interface {
	Run(ctx context.Context, t target.BuildTarget) Result
}

type Progress struct {
	Current   string
	Completed []timing.PackageRecord
}

type ProgressExecutor interface {
	RunWithProgress(ctx context.Context, t target.BuildTarget, onProgress func(Progress)) Result
}

type Result struct {
	Target    string
	StartedAt time.Time
	EndedAt   time.Time
	Timing    timing.Result
	Stderr    string
	Advice    []string
	Err       error
}

func (r Runner) Run(ctx context.Context, t target.BuildTarget) Result {
	return r.RunWithProgress(ctx, t, nil)
}

func (r Runner) RunWithProgress(ctx context.Context, t target.BuildTarget, onProgress func(Progress)) Result {
	now := r.Now
	if now == nil {
		now = time.Now
	}

	start := now()
	eventsFile, err := os.CreateTemp("", "gbwatch-events-*.jsonl")
	if err != nil {
		return Result{
			StartedAt: start,
			EndedAt:   now(),
			Err:       fmt.Errorf("create event file: %w", err),
		}
	}
	eventsPath := eventsFile.Name()
	eventsFile.Close()
	defer os.Remove(eventsPath)

	selfPath, err := os.Executable()
	if err != nil {
		return Result{
			StartedAt: start,
			EndedAt:   now(),
			Err:       fmt.Errorf("resolve executable path: %w", err),
		}
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
		return Result{
			Target:    t.BuildArg,
			StartedAt: start,
			EndedAt:   now(),
			Err:       fmt.Errorf("start go build: %w", err),
		}
	}

	var waitErr error
	if onProgress == nil {
		waitErr = cmd.Wait()
	} else {
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		ticker := time.NewTicker(120 * time.Millisecond)
		defer ticker.Stop()
		progress := progressReader{durations: make(map[string]time.Duration)}
		for {
			select {
			case waitErr = <-done:
				progress.read(eventsPath, onProgress)
				progress.finish(onProgress)
				goto buildFinished
			case <-ticker.C:
				progress.read(eventsPath, onProgress)
			}
		}
	}
buildFinished:
	end := now()

	durations, parseErr := readToolExecEvents(eventsPath, packageFilter(t.ModulePath, r.IncludeAllPackages))

	result := Result{
		Target:    t.BuildArg,
		StartedAt: start,
		EndedAt:   end,
		Timing:    timing.FromDurations(durations, end.Sub(start)),
		Stderr:    strings.TrimSpace(stderr.String()),
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

type progressReader struct {
	offset    int64
	current   string
	durations map[string]time.Duration
	completed []timing.PackageRecord
}

func (p *progressReader) read(path string, onProgress func(Progress)) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	if _, err := file.Seek(p.offset, io.SeekStart); err != nil {
		return
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return
	}
	lastNewline := bytes.LastIndexByte(data, '\n')
	if lastNewline < 0 {
		return
	}
	p.offset += int64(lastNewline + 1)
	for _, line := range bytes.Split(data[:lastNewline], []byte{'\n'}) {
		var event toolExecEvent
		if err := json.Unmarshal(line, &event); err != nil || event.ImportPath == "" {
			continue
		}
		if event.Phase == "start" {
			if event.ImportPath != p.current {
				p.completeCurrent()
				p.current = event.ImportPath
				onProgress(Progress{Current: p.current, Completed: append([]timing.PackageRecord(nil), p.completed...)})
			}
		} else {
			p.durations[event.ImportPath] += event.DurationNS
		}
	}
}

func (p *progressReader) completeCurrent() {
	if p.current == "" || p.durations[p.current] <= 0 {
		return
	}
	p.completed = append(p.completed, timing.PackageRecord{ImportPath: p.current, Duration: p.durations[p.current]})
	if len(p.completed) > MaxProgressPackages {
		p.completed = p.completed[len(p.completed)-MaxProgressPackages:]
	}
}

func (p *progressReader) finish(onProgress func(Progress)) {
	p.completeCurrent()
	p.current = ""
	onProgress(Progress{Completed: append([]timing.PackageRecord(nil), p.completed...)})
}

type ExampleRunner struct {
	Now                Clock
	IncludeAllPackages bool
}

func (r ExampleRunner) Run(ctx context.Context, t target.BuildTarget) Result {
	_ = ctx

	now := r.Now
	if now == nil {
		now = func() time.Time {
			return time.Date(2026, 1, 2, 15, 4, 5, 0, time.UTC)
		}
	}
	start := now()

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
	end := start.Add(total)

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
		Target:    targetArg,
		StartedAt: start,
		EndedAt:   end,
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
