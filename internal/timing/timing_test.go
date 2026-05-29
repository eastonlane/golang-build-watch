package timing

import (
	"testing"
	"time"
)

func TestComputeDurations(t *testing.T) {
	start := time.Unix(0, 0)
	events := []PackageRecord{
		{ImportPath: "a", Completed: start.Add(2 * time.Second)},
		{ImportPath: "b", Completed: start.Add(5 * time.Second)},
	}
	end := start.Add(7 * time.Second)

	result := Compute(start, end, events)

	if len(result.Packages) != 2 {
		t.Fatalf("len(Packages) = %d, want 2", len(result.Packages))
	}

	if result.Packages[0].ImportPath != "b" || result.Packages[0].Duration != 3*time.Second {
		t.Fatalf("first package = %+v, want b with 3s", result.Packages[0])
	}
	if result.Packages[1].ImportPath != "a" || result.Packages[1].Duration != 2*time.Second {
		t.Fatalf("second package = %+v, want a with 2s", result.Packages[1])
	}
	if result.TotalDuration != 7*time.Second {
		t.Fatalf("TotalDuration = %s, want 7s", result.TotalDuration)
	}
}
