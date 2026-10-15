package build

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const toolExecImportPathEnv = "TOOLEXEC_IMPORTPATH"

type toolExecEvent struct {
	ImportPath string        `json:"import_path"`
	Tool       string        `json:"tool"`
	Phase      string        `json:"phase,omitempty"`
	DurationNS time.Duration `json:"duration_ns"`
}

func RunToolExec(args []string) error {
	fs := flag.NewFlagSet("__toolexec", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})

	var eventsFile string
	fs.StringVar(&eventsFile, "events-file", "", "event file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	toolArgs := fs.Args()
	if eventsFile == "" {
		return fmt.Errorf("missing --events-file")
	}
	if len(toolArgs) == 0 {
		return fmt.Errorf("missing tool command")
	}

	toolPath := toolArgs[0]
	cmd := exec.Command(toolPath, toolArgs[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	importPath := strings.TrimSpace(os.Getenv(toolExecImportPathEnv))
	if importPath != "" {
		if err := appendToolExecEvent(eventsFile, toolExecEvent{ImportPath: importPath, Phase: "start"}); err != nil {
			return err
		}
	}

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if importPath != "" {
		if writeErr := appendToolExecEvent(eventsFile, toolExecEvent{
			ImportPath: importPath,
			Tool:       filepath.Base(toolPath),
			Phase:      "end",
			DurationNS: duration,
		}); writeErr != nil && err == nil {
			err = writeErr
		}
	}

	return err
}

func appendToolExecEvent(path string, event toolExecEvent) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open event file: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	return nil
}

func readToolExecEvents(path string, include func(string) bool) (map[string]time.Duration, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open event file: %w", err)
	}
	defer file.Close()

	durations := make(map[string]time.Duration)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event toolExecEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("decode tool event: %w", err)
		}
		if event.Phase == "start" || !include(event.ImportPath) {
			continue
		}
		durations[event.ImportPath] += event.DurationNS
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan event file: %w", err)
	}
	return durations, nil
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
