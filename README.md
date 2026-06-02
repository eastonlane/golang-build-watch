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
gbwatch . --format toml --output-path=./reports/build.toml
gbwatch --example
```

Common flags:

- `--target`: build target relative to the module root
- `--all-packages`: include packages outside the current module
- `--debug`: enable debug logging
- `--config`: load a specific config file
- `--format`: `pretty`, `json`, `toml`, or `csv`
- `--output-path`, `-o`: write the rendered output to a file instead of stdout
  - pass `-o` with no value to use an auto-generated filename
  - pass `--output-path=<path>` or `-o=<path>` to use that path as the base name, with a timestamp suffix added automatically
  - for `json`, `toml`, and `csv`, gbwatch rewrites a missing or mismatched extension to the format's canonical extension
  - for `pretty`, gbwatch keeps the user-provided extension as-is

#### JSON output structure

Raw formats are intended for shell interaction and scripts. JSON field names are stable.

Example shape:

```json
{
  "target": "./...",
  "packages": [
    {
      "import_path": "example.com/project/internal/service",
      "duration_ms": 540
    }
  ],
  "total_duration_ms": 1150,
  "status": "success",
  "stderr": "optional human note",
  "advice": [
    "optional human hint"
  ],
  "error": "optional error string"
}
```

Fields:

- `target`: normalized build target actually used
- `packages`: sorted package timing list
- `packages[].import_path`: package import path
- `packages[].duration_ms`: estimated duration in milliseconds
- `total_duration_ms`: total measured duration in milliseconds
- `status`: `success` or `failed`
- `stderr`: optional human-oriented note
- `advice`: optional human-oriented hints
- `error`: optional build error string

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
output_format = "pretty"
output_path = "./reports/build.toml"
```

Supported `output_format` values:

- `pretty`
- `json`
- `toml`
- `csv`

Behavior:

- if `output_path` is empty, output is written to stdout
- if `-o` is passed with no value, gbwatch writes to an auto-generated file name in the current working directory where `gbwatch` is run
- the auto-generated file name contains a leading text (`gbwatch`), module name, target, and timestamp
- in the auto-generated file name, target `./...` becomes `all-packages`, target `.` becomes `root-package`, and relative targets such as `./cmd/server` become `cmd-server`
- if `--output-path=<path>` or `output_path` is set, gbwatch appends a timestamp to the file name before writing so reruns do not overwrite earlier results
- for `json`, `toml`, and `csv`, gbwatch rewrites the explicit file extension to match the selected format when needed
- for `pretty`, gbwatch preserves the explicit file name and extension and only adds the timestamp
- `output_format = "pretty"` gives the human-readable report
- `output_format = "json"`, `toml`, and `csv` are convenient for later analytics
- in file mode, only a successful run with captured package timing rows writes a file; failed builds or empty timing results cancel file output and print a pretty advisory report to the console instead

## 4. Note

- Package timings are measured from wrapped Go tool invocations and aggregated per package. They are much closer to actual build cost than `go build -v`-style output timing, but they still do not represent every possible internal compiler phase separately.
- `gbwatch` forces sequential builds with `-p=1` so timing deltas are easier to interpret.
- If no build output is received, Go may have served everything from cache. In that case, try:

```bash
go clean -cache
```
