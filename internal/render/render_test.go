package render_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"golang-build-watch/internal/build"
	"golang-build-watch/internal/render"
	"golang-build-watch/internal/timing"
)

func TestTableRenderer(t *testing.T) {
	result := build.Result{
		Target: ".*",
		Timing: timing.Result{
			Packages: []timing.PackageRecord{
				{ImportPath: "example.com/app", Duration: 2 * time.Second},
			},
			TotalDuration: 3 * time.Second,
		},
	}

	output, err := (render.TableRenderer{}).Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	for _, needle := range []string{"GBWATCH REPORT", "Target   : .*", "Status   : success", "#", "example.com/app", "2s"} {
		if !strings.Contains(output, needle) {
			t.Fatalf("output missing %q:\n%s", needle, output)
		}
	}
}

func TestTableRendererShowsAdvice(t *testing.T) {
	result := build.Result{
		Target: "cmd/server",
		Timing: timing.Result{
			TotalDuration: 0,
		},
		Advice: []string{"try `go clean -cache`"},
	}

	output, err := (render.TableRenderer{}).Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	for _, needle := range []string{"No package timing rows were captured.", "Hints:", "go clean -cache"} {
		if !strings.Contains(output, needle) {
			t.Fatalf("output missing %q:\n%s", needle, output)
		}
	}
}

func TestJSONRenderer(t *testing.T) {
	result := build.Result{
		Target: "cmd/server",
		Timing: timing.Result{
			Packages: []timing.PackageRecord{
				{ImportPath: "example.com/app", Duration: 2500 * time.Millisecond},
			},
			TotalDuration: 3 * time.Second,
		},
		Stderr: "note",
		Advice: []string{"hint"},
		Err:    errors.New("build failed"),
	}

	output, err := (render.JSONRenderer{}).Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if payload["status"] != "failed" {
		t.Fatalf("status = %v, want failed", payload["status"])
	}
	if payload["stderr"] != "note" {
		t.Fatalf("stderr = %v, want note", payload["stderr"])
	}
	if payload["target"] != "cmd/server" {
		t.Fatalf("target = %v, want cmd/server", payload["target"])
	}
	if payload["error"] != "build failed" {
		t.Fatalf("error = %v, want build failed", payload["error"])
	}

	packages, ok := payload["packages"].([]any)
	if !ok || len(packages) != 1 {
		t.Fatalf("packages = %#v, want 1 entry", payload["packages"])
	}
}

func TestTableRendererShowsFailedBuildNotice(t *testing.T) {
	result := build.Result{
		Target: "cmd/server",
		Timing: timing.Result{
			Packages: []timing.PackageRecord{
				{ImportPath: "example.com/app", Duration: time.Second},
			},
			TotalDuration: 2 * time.Second,
		},
		Err: errors.New("go build failed: exit status 1"),
	}

	output, err := (render.TableRenderer{}).Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	for _, needle := range []string{"Status   : failed", "Build    : failed; timings below may be partial"} {
		if !strings.Contains(output, needle) {
			t.Fatalf("output missing %q:\n%s", needle, output)
		}
	}
}
