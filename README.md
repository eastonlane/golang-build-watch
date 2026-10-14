# Golang-Build-Watch

`gbwatch` measures per-package Go build cost by wrapping build tool invocations and aggregating timing data per package. It helps you understand where build time is being spent for a package, a directory, or a pattern such as `./...`.

This project is almost entirely developed by Codex, with iterative direction and review from the repository owner.

## 1. Introduction

Main capabilities:

- Approximate per-package Go build timing
- Human-friendly table output
- Machine-friendly JSON output for shell pipelines
- Module-only output or all compiled packages
- Debug logging for raw build events
- Config file support through `~/.gbwatch.toml`

## 2. Getting Start

### 2.1. Install

#### `go install` (TBD)

The final `go install` path is not published yet.

```bash
go install <TBD>
```

#### Build from source

```bash
make build
```

This writes the binary to:

```text
bin/gbwatch
```

Override the embedded build version explicitly:

```bash
make build VERSION=v0.1.0
```

If `VERSION` is not set, `make` uses the current short git commit hash. When tracked files have changes or untracked files are present, `make` appends `-dirty` to the embedded version, including when `VERSION` is set explicitly.

Cross-platform builds:

```bash
make build-all
```

Artifacts:

- `bin/gbwatch-windows-amd64.exe`
- `bin/gbwatch-linux-amd64`
- `bin/gbwatch-darwin-amd64`
- `bin/gbwatch-darwin-arm64`

### 2.2. How to use

Basic form:

```bash
gbwatch <module-root> --target ./...
```

Examples:

```bash
gbwatch . --target .
gbwatch . --target ./...
gbwatch . --target ./cmd/server
gbwatch . --target ./... --all-packages
gbwatch . --target ./... --format json
gbwatch . --target ./... --debug
gbwatch . --format json
gbwatch . --format csv
gbwatch . --report-name timing.json
gbwatch . --report-name timing -f json
gbwatch . --report-name=./reports/build.csv
gbwatch help
gbwatch help format
gbwatch help format csv
gbwatch help csv
gbwatch --example
```

Common flags:

- `--target`: build target relative to the module root
- `--all-packages`: include packages outside the current module
- `--debug`: enable debug logging
- `--version`: print the embedded build version and exit
- `--config`: load a specific config file
- `--format`: `pretty`, `json`, or `csv`
- `--report-name`, `-r`: name a report file instead of writing to stdout
  - pass `-r` with no value to use an auto-generated filename
  - pass `--report-name=<name>` or `-r=<name>` to set its base name, optionally including a directory
  - `.json`, `.csv`, and `.txt` select JSON, CSV, and pretty output when `--format` is absent
  - `--report-name timing.json` and `--report-name timing -f json` produce equivalent JSON reports
  - a conflicting suffix and explicit format, such as `-r report.json -f csv`, is an error
  - file names use `{report-name}-{module-name}-{build-start-time}{UTC-offset}.{extension}`, using the machine's local timezone (for example, `timing-my-module-20261014-152109+0800.json`)
  - if a timestamped name already exists, gbwatch adds a numeric suffix rather than replacing the earlier report
  - for `json` and `csv`, gbwatch adds the format's extension when the name has none or an unrecognized extension
  - for `pretty`, gbwatch keeps the user-provided extension as-is
- `gbwatch help [topic]`: show topic help; `gbwatch help` is the same as `gbwatch -h`
  - use `gbwatch help format` to browse formats; `gbwatch help csv` and `gbwatch help format csv` both describe the CSV file layout
  - unknown help topics return an error and a nonzero exit code

#### JSON output structure

Raw formats are intended for shell interaction and scripts. JSON field names are stable.

Example shape:

```json
{
  "total_duration_ms": 65000,
  "total_duration_human": "1 minute 5 seconds",
  "target": "./...",
  "started_at": "2026-01-02T15:04:05Z",
  "ended_at": "2026-01-02T15:05:10Z",
  "status": "success",
  "stderr": "optional human note",
  "advice": [
    "optional human hint"
  ],
  "packages": [
    {
      "import_path": "example.com/project/internal/service",
      "duration_ms": 540,
      "duration_human": "540 milliseconds"
    }
  ]
}
```

Fields:

- `target`: normalized build target actually used
- `total_duration_ms`: total measured duration in milliseconds
- `total_duration_human`: readable duration formatted with `github.com/hako/durafmt`, such as `4 minutes 45 seconds`
- `packages`: sorted package timing list
- `started_at`: wall-clock timestamp when the build attempt began, in RFC3339 format with its recorded timezone offset
- `ended_at`: wall-clock timestamp when the build attempt ended, in RFC3339 format with its recorded timezone offset
- `packages[].import_path`: package import path
- `packages[].duration_ms`: estimated duration in milliseconds
- `packages[].duration_human`: package duration in readable units, including remaining seconds; `duration_ms` retains the exact millisecond value
- `status`: `success` or `failed`
- `stderr`: optional human-oriented note
- `advice`: optional human-oriented hints
- `error`: optional build error string on failed builds

#### CSV file output

When writing CSV to a file, gbwatch writes a matched pair:

```text
timing-my-module-20261014-151002+0800.json
timing-my-module-20261014-151002+0800.csv
```

The JSON file contains build metadata (target, total duration, start and end times, status, and any stderr or advice) without package rows. The CSV file contains only package timing rows with `import_path,duration_ms,duration_human`. Both files use the same stem and, on a filename collision, the same numeric suffix. CSV sent to stdout contains only package rows.

## 3. Configuration

Default config path:

```text
~/.gbwatch.toml
```

CLI flags override config-file values.

Example config:

```toml
[settings]
target = "./..."
all_packages = false
debug = false
report_name = "./reports/build.json"
```

Supported `output_format` values:

- `pretty`
- `json`
- `csv`

Behavior:

- if `report_name` is empty and `--report-name` is absent, output is written to stdout
- if `-r` is passed with no value, gbwatch writes to an auto-generated file name in the current working directory where `gbwatch` is run
- the auto-generated file name contains `gbwatch`, the module name, and the build start time in the local timezone with its numeric UTC offset
- if `--report-name=<name>` or `report_name` is set, gbwatch appends the module name, local build start time, and UTC offset to the file name before writing
- the old config key `output_path` remains readable for existing config files; `report_name` takes precedence when both are present
- if that name already exists, gbwatch adds `-2`, `-3`, and so on before the extension; CSV companions share the suffix
- for `json` and `csv`, gbwatch replaces an unrecognized extension with the selected format's extension; a recognized conflicting extension is an error
- for `pretty`, gbwatch preserves the explicit extension and adds the module name and start time
- `output_format = "pretty"` gives the human-readable report
- `output_format = "json"` and `csv` are convenient for later analytics
- CSV contains package rows with `duration_ms` and `duration_human`; in file mode a companion JSON report holds the build metadata
- in file mode, only a successful run with captured package timing rows writes a file; failed builds or empty timing results cancel file output and print a pretty advisory report to the console instead

## 4. Note

- Package timings are measured from wrapped Go tool invocations and aggregated per package. They are much closer to actual build cost than `go build -v`-style output timing, but they still do not represent every possible internal compiler phase separately.
- `gbwatch` forces sequential builds with `-p=1` so timing deltas are easier to interpret.
- If no build output is received, Go may have served everything from cache. In that case, try:

```bash
go clean -cache
```
