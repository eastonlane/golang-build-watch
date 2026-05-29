# Golang-Build-Watch

`gbwatch` watches `go build` output and estimates how long each package took to finish building. It helps you understand where build time is being spent for a package, a directory, or a pattern such as `./...`.

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
gbwatch . --target ./... --json
gbwatch . --target ./... --debug
gbwatch --example
```

Common flags:

- `--target`: build target relative to the module root
- `--all-packages`: include packages outside the current module
- `--json`: render structured JSON
- `--debug`: enable debug logging
- `--config`: load a specific config file

#### JSON output structure

`--json` is intended for shell interaction and scripts. The JSON field names are stable.

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
output_format = "table"
```

Supported `output_format` values:

- `table`
- `json`

## 4. Note

- Package timings are inferred from `go build -v` completion output, so they are approximations rather than compiler-internal phase timings.
- `gbwatch` forces sequential builds with `-p=1` so timing deltas are easier to interpret.
- If no build output is received, Go may have served everything from cache. In that case, try:

```bash
go clean -cache
```
