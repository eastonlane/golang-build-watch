package timing

import (
	"sort"
	"time"
)

type PackageRecord struct {
	ImportPath string
	Completed  time.Time
	Duration   time.Duration
}

type Result struct {
	Packages      []PackageRecord
	TotalDuration time.Duration
}

func FromDurations(durations map[string]time.Duration, total time.Duration) Result {
	records := make([]PackageRecord, 0, len(durations))
	for importPath, duration := range durations {
		records = append(records, PackageRecord{
			ImportPath: importPath,
			Duration:   duration,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Duration == records[j].Duration {
			return records[i].ImportPath < records[j].ImportPath
		}
		return records[i].Duration > records[j].Duration
	})

	return Result{
		Packages:      records,
		TotalDuration: total,
	}
}

func Compute(start, end time.Time, events []PackageRecord) Result {
	records := make([]PackageRecord, 0, len(events))
	last := start
	for _, event := range events {
		record := event
		record.Duration = record.Completed.Sub(last)
		records = append(records, record)
		last = record.Completed
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Duration == records[j].Duration {
			return records[i].ImportPath < records[j].ImportPath
		}
		return records[i].Duration > records[j].Duration
	})

	return Result{
		Packages:      records,
		TotalDuration: end.Sub(start),
	}
}
