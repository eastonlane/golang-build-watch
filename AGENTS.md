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
  - `--format pretty|json|toml|csv`
  - `-o`, `--output-path` to write the rendered output to a file
- If `output_path` is empty, output goes to stdout.
- If `-o` is passed without a value, generate a default output file name from leading text, module name, target, and timestamp.
- If `output_path` is set explicitly, append a timestamp suffix before writing so reruns do not overwrite the prior file.
- In file mode, any non-success path, including failed builds and empty package timing results, must cancel file dumping and print pretty advice to the console instead.
- `--example` should keep working without a real module root.

## Config Rules

- Default config path: `~/.gbwatch.toml`
- Supported config keys:
  - `target`
  - `all_packages`
  - `debug`
  - `output_format`
  - `output_path`
- CLI values override config-file values.

## Output Rules

- Pretty output is for humans and may evolve visually.
- JSON/TOML/CSV are for automation and analytics.
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
