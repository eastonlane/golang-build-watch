# AGENTS.md

## Purpose

`gbwatch` measures per-package Go build cost for a module target.

This project is almost entirely developed by Codex, with human direction and review.

## Timing Model

- Package timings are measured through `go build -toolexec`, not `go build -v`.
- Per-package durations are aggregated from wrapped tool invocations using `TOOLEXEC_IMPORTPATH`.
- `Total` is wall-clock build time and may not equal the sum of listed package durations.
- A failed build may still produce partial per-package timing data.

## CLI Rules

- Positional argument: `<module-root>`
- Primary target flag: `--target`
- Output selection:
  - `--format pretty|json|csv`
  - `-r`, `--report-name` to name a report file
- If `report_name` is empty and `--report-name` is absent, output goes to stdout.
- If `-r` is passed without a value, generate a default output file name from leading text, module name, and local build start time with numeric UTC offset.
- If `report_name` is set explicitly, append the module name and local build start time with numeric UTC offset before writing so reruns do not overwrite the prior file.
- A recognized report filename extension selects the format; a conflicting explicit `--format` is an error.
- In CSV file mode, write metadata to `.json` and package timing rows to `.csv` with the same filename stem.
- `gbwatch help` matches `gbwatch -h`; `gbwatch help format csv` describes CSV file output.
- In file mode, any non-success path, including failed builds and empty package timing results, must cancel file dumping and print pretty advice to the console instead.
- `--example` should keep working without a real module root.

## Config Rules

- Default config path: `~/.gbwatch.toml`
- Supported config keys:
  - `target`
  - `all_packages`
  - `debug`
  - `output_format`
  - `report_name` (`output_path` remains a legacy alias)
- CLI values override config-file values.

## Output Rules

- Pretty output is for humans and may evolve visually.
- JSON/CSV are for automation and analytics.
- JSON field names should remain stable unless there is an explicit breaking-change decision.
- Pretty output should clearly mark failed builds as potentially partial.
- When no package timing rows are captured, render an explicit empty-state message.

## Change Guardrails

- Do not reintroduce `-v`-based timing as the primary timing model.
- Keep the hidden `__toolexec` helper path working when changing `cmd/main.go`.
- Preserve `go test ./...` as the baseline verification command.
- Keep the `Makefile` aligned with the shell actually used by `make` in this repo.
- Avoid changing structured output field names casually.

## Dev Workflow

- `make build`
- `make build-all`
- `go test ./...`
