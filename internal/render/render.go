package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/hako/durafmt"

	"golang-build-watch/internal/build"
)

type Renderer interface {
	Render(result build.Result) (string, error)
}

type TableRenderer struct {
}

type JSONRenderer struct {
}

type CSVRenderer struct {
}

type jsonPackage struct {
	ImportPath    string `json:"import_path"`
	DurationMS    int64  `json:"duration_ms"`
	DurationHuman string `json:"duration_human"`
}

type jsonMetadata struct {
	TotalDuration      int64    `json:"total_duration_ms"`
	TotalDurationHuman string   `json:"total_duration_human"`
	Target             string   `json:"target"`
	StartedAt          string   `json:"started_at"`
	EndedAt            string   `json:"ended_at"`
	Status             string   `json:"status"`
	Stderr             string   `json:"stderr,omitempty"`
	Advice             []string `json:"advice,omitempty"`
	Error              string   `json:"error,omitempty"`
}

type jsonResult struct {
	jsonMetadata
	Packages []jsonPackage `json:"packages"`
}

func (r TableRenderer) Render(result build.Result) (string, error) {
	var buf bytes.Buffer
	if _, err := fmt.Fprintln(&buf, "GBWATCH REPORT"); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintln(&buf, "=============="); err != nil {
		return "", err
	}

	status := "success"
	if result.Err != nil {
		status = "failed"
	}
	if _, err := fmt.Fprintf(&buf, "%-8s : %s\n%-8s : %s\n%-8s : %s\n%-8s : %s\n%-8s : %s\n%-8s : %d\n\n",
		"Total",
		formatDuration(result.Timing.TotalDuration),
		"Target",
		result.Target,
		"Status",
		status,
		"Started",
		formatPrettyTimestamp(result.StartedAt),
		"Ended",
		formatPrettyTimestamp(result.EndedAt),
		"Packages",
		len(result.Timing.Packages),
	); err != nil {
		return "", err
	}
	if result.Err != nil {
		if _, err := fmt.Fprintf(&buf, "Build    : failed; timings below may be partial\n\n"); err != nil {
			return "", err
		}
	}

	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	if len(result.Timing.Packages) > 0 {
		if _, err := fmt.Fprintf(tw, "#\t%s\t%s\n", "DURATION", "PACKAGE"); err != nil {
			return "", err
		}

		for i, pkg := range result.Timing.Packages {
			if _, err := fmt.Fprintf(tw, "%d\t%s\t%s\n", i+1, formatDuration(pkg.Duration), pkg.ImportPath); err != nil {
				return "", err
			}
		}

		if err := tw.Flush(); err != nil {
			return "", err
		}
	} else {
		if _, err := fmt.Fprintf(&buf, "%s\n", "No package timing rows were captured."); err != nil {
			return "", err
		}
	}

	if len(result.Advice) > 0 {
		if _, err := fmt.Fprintf(&buf, "\n%s:\n", "Hints"); err != nil {
			return "", err
		}
		for _, advice := range result.Advice {
			if _, err := fmt.Fprintf(&buf, "  - %s\n", advice); err != nil {
				return "", err
			}
		}
	}

	return buf.String(), nil
}

func (r JSONRenderer) Render(result build.Result) (string, error) {
	packages := make([]jsonPackage, 0, len(result.Timing.Packages))
	for _, pkg := range result.Timing.Packages {
		packages = append(packages, jsonPackage{
			ImportPath:    pkg.ImportPath,
			DurationMS:    pkg.Duration.Milliseconds(),
			DurationHuman: formatDuration(pkg.Duration),
		})
	}

	payload := jsonResult{
		jsonMetadata: metadataForResult(result),
		Packages:     packages,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data) + "\n", nil
}

type MetadataJSONRenderer struct{}

func (r MetadataJSONRenderer) Render(result build.Result) (string, error) {
	data, err := json.MarshalIndent(metadataForResult(result), "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

func metadataForResult(result build.Result) jsonMetadata {
	metadata := jsonMetadata{
		TotalDuration:      result.Timing.TotalDuration.Milliseconds(),
		TotalDurationHuman: formatDuration(result.Timing.TotalDuration),
		Target:             result.Target,
		StartedAt:          formatTimestamp(result.StartedAt),
		EndedAt:            formatTimestamp(result.EndedAt),
		Status:             statusForResult(result),
		Stderr:             result.Stderr,
		Advice:             result.Advice,
	}
	if result.Err != nil {
		metadata.Error = result.Err.Error()
	}
	return metadata
}

func (r CSVRenderer) Render(result build.Result) (string, error) {
	var buf bytes.Buffer
	buf.WriteString("import_path,duration_ms,duration_human\n")
	for _, pkg := range result.Timing.Packages {
		buf.WriteString(strconv.Quote(pkg.ImportPath))
		buf.WriteByte(',')
		buf.WriteString(strconv.FormatInt(pkg.Duration.Milliseconds(), 10))
		buf.WriteByte(',')
		buf.WriteString(strconv.Quote(formatDuration(pkg.Duration)))
		buf.WriteByte('\n')
	}
	return buf.String(), nil
}

func formatTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339Nano)
}

func formatPrettyTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format("2006-01-02 15:04:05.000 -07:00")
}

func formatDuration(duration time.Duration) string {
	return durafmt.Parse(duration.Truncate(time.Millisecond)).String()
}

func statusForResult(result build.Result) string {
	if result.Err != nil {
		return "failed"
	}
	return "success"
}
