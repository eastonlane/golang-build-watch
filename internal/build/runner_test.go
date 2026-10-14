package build_test

import (
	"context"
	"testing"
	"time"

	"golang-build-watch/internal/build"
	"golang-build-watch/internal/target"
)

func TestExampleRunner(t *testing.T) {
	start := time.Date(2026, 6, 3, 10, 11, 12, 0, time.UTC)
	result := build.ExampleRunner{
		Now:                func() time.Time { return start },
		IncludeAllPackages: true,
	}.Run(context.Background(), target.BuildTarget{})

	if result.Err != nil {
		t.Fatalf("Run() error = %v", result.Err)
	}
	if len(result.Timing.Packages) == 0 {
		t.Fatal("expected mock packages")
	}
	if result.Target != "./..." {
		t.Fatalf("Target = %q, want ./...", result.Target)
	}
	if result.Stderr == "" {
		t.Fatal("expected example note")
	}
	if !result.StartedAt.Equal(start) {
		t.Fatalf("StartedAt = %v, want %v", result.StartedAt, start)
	}
	if !result.EndedAt.Equal(start.Add(result.Timing.TotalDuration)) {
		t.Fatalf("EndedAt = %v, want %v", result.EndedAt, start.Add(result.Timing.TotalDuration))
	}
}
