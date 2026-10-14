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
		Target:    ".*",
		StartedAt: time.Date(2026, 6, 3, 8, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 6, 3, 8, 0, 3, 0, time.UTC),
		Timing: timing.Result{
			Packages: []timing.PackageRecord{
				{ImportPath: "example.com/app", Duration: 2 * time.Second},
			},
			TotalDuration: time.Minute + 5*time.Second,
		},
	}

	output, err := (render.TableRenderer{}).Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	for _, needle := range []string{"GBWATCH REPORT", "Target   : .*", "Status   : success", "Started  : 2026-06-03 08:00:00.000 +00:00", "Ended    : 2026-06-03 08:00:03.000 +00:00", "#", "example.com/app", "2 seconds"} {
		if !strings.Contains(output, needle) {
			t.Fatalf("output missing %q:\n%s", needle, output)
		}
	}
	if !strings.Contains(output, "Total    : 1 minute 5 seconds") || strings.Index(output, "Total    :") > strings.Index(output, "Target   :") {
		t.Fatalf("total duration is not first: %s", output)
	}
}

func TestRenderersPreserveTimestampOffset(t *testing.T) {
	zone := time.FixedZone("UTC+8", 8*60*60)
	result := build.Result{
		StartedAt: time.Date(2026, 10, 14, 15, 9, 59, 921954300, zone),
		EndedAt:   time.Date(2026, 10, 14, 15, 10, 0, 993609000, zone),
	}
	tests := []struct {
		name     string
		renderer render.Renderer
		started  string
		ended    string
	}{
		{"pretty", render.TableRenderer{}, "Started  : 2026-10-14 15:09:59.921 +08:00", "Ended    : 2026-10-14 15:10:00.993 +08:00"},
		{"json", render.JSONRenderer{}, `"started_at": "2026-10-14T15:09:59.9219543+08:00"`, `"ended_at": "2026-10-14T15:10:00.993609+08:00"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.renderer.Render(result)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if !strings.Contains(output, tt.started) || !strings.Contains(output, tt.ended) {
				t.Fatalf("output missing timestamps:\n%s", output)
			}
		})
	}
}

func TestTableRendererShowsAdvice(t *testing.T) {
	result := build.Result{
		Target:    "cmd/server",
		StartedAt: time.Date(2026, 6, 3, 8, 1, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 6, 3, 8, 1, 0, 0, time.UTC),
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
		Target:    "cmd/server",
		StartedAt: time.Date(2026, 6, 3, 8, 2, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 6, 3, 8, 2, 3, 0, time.UTC),
		Timing: timing.Result{
			Packages: []timing.PackageRecord{
				{ImportPath: "example.com/app", Duration: 2500 * time.Millisecond},
			},
			TotalDuration: time.Minute + 5*time.Second,
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
	if payload["total_duration_ms"] != float64(65000) || payload["total_duration_human"] != "1 minute 5 seconds" {
		t.Fatalf("total duration fields = %v, %v", payload["total_duration_ms"], payload["total_duration_human"])
	}
	if !strings.HasPrefix(output, "{\n  \"total_duration_ms\": 65000,\n  \"total_duration_human\": \"1 minute 5 seconds\",") {
		t.Fatalf("total duration is not first: %s", output)
	}
	if payload["stderr"] != "note" {
		t.Fatalf("stderr = %v, want note", payload["stderr"])
	}
	if payload["target"] != "cmd/server" {
		t.Fatalf("target = %v, want cmd/server", payload["target"])
	}
	if payload["started_at"] != "2026-06-03T08:02:00Z" {
		t.Fatalf("started_at = %v, want 2026-06-03T08:02:00Z", payload["started_at"])
	}
	if payload["ended_at"] != "2026-06-03T08:02:03Z" {
		t.Fatalf("ended_at = %v, want 2026-06-03T08:02:03Z", payload["ended_at"])
	}
	if payload["error"] != "build failed" {
		t.Fatalf("error = %v, want build failed", payload["error"])
	}

	packages, ok := payload["packages"].([]any)
	if !ok || len(packages) != 1 {
		t.Fatalf("packages = %#v, want 1 entry", payload["packages"])
	}
	if pkg := packages[0].(map[string]any); pkg["duration_human"] != "2 seconds 500 milliseconds" {
		t.Fatalf("package duration_human = %v, want 2 seconds 500 milliseconds", pkg["duration_human"])
	}
}

func TestMetadataJSONRendererOmitsPackageRows(t *testing.T) {
	result := build.Result{
		Target: "./cmd",
		Timing: timing.Result{
			Packages:      []timing.PackageRecord{{ImportPath: "example.com/app", Duration: 2 * time.Second}},
			TotalDuration: 3 * time.Second,
		},
	}
	output, err := (render.MetadataJSONRenderer{}).Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := payload["packages"]; ok {
		t.Fatalf("metadata contains package rows: %s", output)
	}
	if payload["target"] != "./cmd" || payload["total_duration_ms"] != float64(3000) {
		t.Fatalf("metadata missing build summary: %s", output)
	}
}

func TestCSVRenderer(t *testing.T) {
	result := build.Result{
		Target:    "cmd/server",
		StartedAt: time.Date(2026, 6, 3, 8, 4, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 6, 3, 8, 4, 3, 0, time.UTC),
		Timing: timing.Result{
			Packages: []timing.PackageRecord{
				{ImportPath: "example.com/app", Duration: 2500 * time.Millisecond},
			},
		},
	}

	output, err := (render.CSVRenderer{}).Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	for _, needle := range []string{
		"import_path,duration_ms,duration_human",
		"\"example.com/app\",2500,\"2 seconds 500 milliseconds\"",
	} {
		if !strings.Contains(output, needle) {
			t.Fatalf("output missing %q:\n%s", needle, output)
		}
	}
}

func TestJSONRendererReadableDurationKeepsRemainder(t *testing.T) {
	result := build.Result{Timing: timing.Result{
		TotalDuration: time.Minute + 5*time.Second + 123456789*time.Nanosecond,
		Packages: []timing.PackageRecord{
			{ImportPath: "example.com/app", Duration: time.Second + 456789*time.Nanosecond},
		},
	}}

	output, err := (render.JSONRenderer{}).Render(result)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	var payload struct {
		TotalDurationMS    int64  `json:"total_duration_ms"`
		TotalDurationHuman string `json:"total_duration_human"`
		Packages           []struct {
			DurationMS    int64  `json:"duration_ms"`
			DurationHuman string `json:"duration_human"`
		} `json:"packages"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.TotalDurationMS != 65123 || payload.TotalDurationHuman != "1 minute 5 seconds 123 milliseconds" {
		t.Fatalf("total duration = %d, %q", payload.TotalDurationMS, payload.TotalDurationHuman)
	}
	if payload.Packages[0].DurationMS != 1000 || payload.Packages[0].DurationHuman != "1 second" {
		t.Fatalf("package duration = %d, %q", payload.Packages[0].DurationMS, payload.Packages[0].DurationHuman)
	}
}

func TestRenderersKeepSecondsBeyondOneMinute(t *testing.T) {
	result := build.Result{Timing: timing.Result{
		TotalDuration: 4*time.Minute + 45*time.Second,
		Packages: []timing.PackageRecord{
			{ImportPath: "example.com/app", Duration: 4*time.Minute + 45*time.Second},
		},
	}}
	tests := []struct {
		name     string
		renderer render.Renderer
		want     string
	}{
		{"pretty", render.TableRenderer{}, "Total    : 4 minutes 45 seconds"},
		{"json", render.JSONRenderer{}, `"total_duration_human": "4 minutes 45 seconds"`},
		{"csv", render.CSVRenderer{}, `"example.com/app",285000,"4 minutes 45 seconds"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.renderer.Render(result)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if !strings.Contains(output, tt.want) {
				t.Fatalf("output missing %q:\n%s", tt.want, output)
			}
		})
	}
}

func TestTableRendererShowsFailedBuildNotice(t *testing.T) {
	result := build.Result{
		Target:    "cmd/server",
		StartedAt: time.Date(2026, 6, 3, 8, 6, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 6, 3, 8, 6, 2, 0, time.UTC),
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
