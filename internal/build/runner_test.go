package build_test

import (
	"context"
	"testing"

	"golang-build-watch/internal/build"
	"golang-build-watch/internal/target"
)

func TestExampleRunner(t *testing.T) {
	result := build.ExampleRunner{
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
}
