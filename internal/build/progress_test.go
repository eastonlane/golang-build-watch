package build

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestProgressReaderTracksCurrentAndCompletedPackages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	for _, event := range []toolExecEvent{
		{ImportPath: "example.com/a", Phase: "start"},
		{ImportPath: "example.com/a", Phase: "end", DurationNS: time.Second},
		{ImportPath: "example.com/b", Phase: "start"},
		{ImportPath: "example.com/b", Phase: "end", DurationNS: 2 * time.Second},
		{ImportPath: "example.com/c", Phase: "start"},
		{ImportPath: "example.com/c", Phase: "end", DurationNS: 3 * time.Second},
	} {
		if err := appendToolExecEvent(path, event); err != nil {
			t.Fatal(err)
		}
	}

	reader := progressReader{durations: make(map[string]time.Duration)}
	var latest Progress
	reader.read(path, func(progress Progress) { latest = progress })
	if latest.Current != "example.com/c" || len(latest.Completed) != 2 {
		t.Fatalf("progress = %+v", latest)
	}
	if latest.Completed[0].ImportPath != "example.com/a" || latest.Completed[0].Duration != time.Second || latest.Completed[1].ImportPath != "example.com/b" || latest.Completed[1].Duration != 2*time.Second {
		t.Fatalf("completed = %+v", latest.Completed)
	}
	reader.finish(func(progress Progress) { latest = progress })
	if latest.Current != "" || len(latest.Completed) != 3 || latest.Completed[2].ImportPath != "example.com/c" || latest.Completed[2].Duration != 3*time.Second {
		t.Fatalf("final progress = %+v", latest)
	}

	durations, err := readToolExecEvents(path, func(string) bool { return true })
	if err != nil || durations["example.com/a"] != time.Second || durations["example.com/b"] != 2*time.Second || durations["example.com/c"] != 3*time.Second {
		t.Fatalf("durations = %v, error = %v", durations, err)
	}
}

func TestProgressReaderKeepsRecentHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	for i := 0; i < MaxProgressPackages+5; i++ {
		importPath := fmt.Sprintf("example.com/package%d", i)
		for _, event := range []toolExecEvent{
			{ImportPath: importPath, Phase: "start"},
			{ImportPath: importPath, Phase: "end", DurationNS: time.Second},
		} {
			if err := appendToolExecEvent(path, event); err != nil {
				t.Fatal(err)
			}
		}
	}

	reader := progressReader{durations: make(map[string]time.Duration)}
	var latest Progress
	reader.read(path, func(progress Progress) { latest = progress })
	if len(latest.Completed) != MaxProgressPackages || latest.Completed[0].ImportPath != "example.com/package4" || latest.Current != "example.com/package24" {
		t.Fatalf("progress history = %+v", latest)
	}
	reader.finish(func(progress Progress) { latest = progress })
	if len(latest.Completed) != MaxProgressPackages || latest.Completed[0].ImportPath != "example.com/package5" || latest.Completed[MaxProgressPackages-1].ImportPath != "example.com/package24" {
		t.Fatalf("final progress history = %+v", latest)
	}
}
