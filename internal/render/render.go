package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/BurntSushi/toml"

	"golang-build-watch/internal/build"
)

type Renderer interface {
	Render(result build.Result) (string, error)
}

type TableRenderer struct {
}

type JSONRenderer struct {
}

type TOMLRenderer struct {
}

type CSVRenderer struct {
}

type jsonPackage struct {
	ImportPath string `json:"import_path"`
	DurationMS int64  `json:"duration_ms"`
}

type jsonResult struct {
	Target        string        `json:"target"`
	Packages      []jsonPackage `json:"packages"`
	TotalDuration int64         `json:"total_duration_ms"`
	Status        string        `json:"status"`
	Stderr        string        `json:"stderr,omitempty"`
	Advice        []string      `json:"advice,omitempty"`
	Error         string        `json:"error,omitempty"`
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
	if _, err := fmt.Fprintf(&buf, "%-8s : %s\n%-8s : %s\n%-8s : %d\n%-8s : %s\n\n",
		"Target",
		result.Target,
		"Status",
		status,
		"Packages",
		len(result.Timing.Packages),
		"Total",
		result.Timing.TotalDuration,
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
			if _, err := fmt.Fprintf(tw, "%d\t%s\t%s\n", i+1, pkg.Duration, pkg.ImportPath); err != nil {
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
	status := "success"
	if result.Err != nil {
		status = "failed"
	}

	packages := make([]jsonPackage, 0, len(result.Timing.Packages))
	for _, pkg := range result.Timing.Packages {
		packages = append(packages, jsonPackage{
			ImportPath: pkg.ImportPath,
			DurationMS: pkg.Duration.Milliseconds(),
		})
	}

	payload := jsonResult{
		Target:        result.Target,
		Packages:      packages,
		TotalDuration: result.Timing.TotalDuration.Milliseconds(),
		Status:        status,
		Stderr:        result.Stderr,
		Advice:        result.Advice,
	}
	if result.Err != nil {
		payload.Error = result.Err.Error()
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data) + "\n", nil
}

func (r TOMLRenderer) Render(result build.Result) (string, error) {
	payload := jsonResult{
		Target:        result.Target,
		TotalDuration: result.Timing.TotalDuration.Milliseconds(),
		Status:        statusForResult(result),
		Stderr:        result.Stderr,
		Advice:        result.Advice,
	}
	if result.Err != nil {
		payload.Error = result.Err.Error()
	}
	for _, pkg := range result.Timing.Packages {
		payload.Packages = append(payload.Packages, jsonPackage{
			ImportPath: pkg.ImportPath,
			DurationMS: pkg.Duration.Milliseconds(),
		})
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(payload); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r CSVRenderer) Render(result build.Result) (string, error) {
	var buf bytes.Buffer
	buf.WriteString("import_path,duration_ms\n")
	for _, pkg := range result.Timing.Packages {
		buf.WriteString(strconv.Quote(pkg.ImportPath))
		buf.WriteByte(',')
		buf.WriteString(strconv.FormatInt(pkg.Duration.Milliseconds(), 10))
		buf.WriteByte('\n')
	}
	return buf.String(), nil
}

func statusForResult(result build.Result) string {
	if result.Err != nil {
		return "failed"
	}
	return "success"
}
