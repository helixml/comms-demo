# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`comms-demo` — a throwaway Go webapp that mocks communication tools (Slack, Discord, email, SMS, etc.) by exchanging webhooks. The goal is a self-contained sandbox for exercising integrations end-to-end without hitting real third-party services.

Because this is a demo, prefer the smallest thing that works over production rigour. Skip auth, skip persistence-by-default, skip operational features that aren't being demonstrated. Add complexity only when a specific demo scenario needs it.

**Current year: 2026** — include "2026" in web searches for documentation and library APIs.

## Repository Status

The project is essentially empty: no `go.mod`, no `Makefile`, no Go source yet. Everything below describes the conventions to follow once code starts landing — keep this file in step with the actual repo as it grows. If a section here describes infrastructure that does not yet exist, treat it as a target, not a fact.

## Build, Test, and Check

Once a `Makefile` lands, **prefer `make` targets over raw shell commands**. The Makefile is the canonical record of how the project builds, tests, lints, and runs locally; ad-hoc `go run` / `go test` / `golangci-lint` invocations drift from it.

If you find yourself reaching for a multi-step shell incantation to build, run, test, format, lint, or seed local state — **add a `make` target for it instead**, then call that target. Keep targets discoverable via `make help`.

Expected targets (add as needed):

```
make build       # compile binaries into ./bin
make run         # run the webapp locally with sensible defaults
make test        # go test ./... with -race and -count=1
make check       # fmt + vet + lint + test (may modify files)
make ci          # CI-safe variant: fmt-check instead of fmt, no writes
make clean       # kill running servers and remove build artefacts
```

`make check` is for local use (runs `goimports -w`). `make ci` runs `fmt-check` instead and fails on drift without touching files.

## Shipping Code

Before committing:

1. Run `make check` (or at minimum `make test` for affected packages). Do not commit code that has not been validated.
2. Fix failures before committing — do not skip or work around them.

Commits and PRs use **Conventional Commits**:

- Prefix: `feat:`, `fix:`, `docs:`, `refactor:`, `chore:`, `test:`, etc.
- Example: `feat: add slack webhook receiver`
- PR titles follow the same format.

When pushing further commits to an existing PR, update the PR title and description to reflect the full set of changes in the branch.

## Go

- Fail fast: `return fmt.Errorf("...: %w", err)` — never log and continue.
- Error on missing configuration — fail with an error, don't log a warning and continue.
- Use structs, not `map[string]interface{}`.
- Use gomock, not `testify/mock`.
- No fallbacks — one approach, no fallback code paths.
- No type aliases — update all references when moving or renaming types.
- No panics — return errors; rewrite methods to support error returns if needed.
- Log errors once at the top level — domain code returns errors; only handlers/workers log them.

## Testing

When the user says "tdd", follow red-green strictly:

1. **Red**: Write a failing test. Run it, confirm it fails.
2. **Green**: Minimal fix. Run test, confirm it passes.
3. Run the full suite for regressions.

Practices:

- **Tests live next to the code**: `foo.go` → `foo_test.go` in the same directory. Use package `foo` for whitebox tests and `foo_test` only when the test must exercise the public API in isolation (e.g. to avoid import cycles).
- **Table-driven tests** for anything with multiple input cases. Use `t.Run(name, ...)` per case so failures name the case.
- **`t.Parallel()`** in tests that don't share global state.
- **Race detector is always on** (`make test` adds `-race`). Treat race failures as hard bugs, never flakes.
- **`-count=1`** so tests never use the build cache — all runs are fresh.
- **Mocks**: use `gomock` generated via `mockgen`. Place generated mocks in `mocks/<pkg>/`. Prefer hand-rolled fakes where the interface is small enough that a mock adds no value.
- **Fixtures**: put reusable test data under `testdata/` (Go ignores it during builds).
- **Coverage**: run `make test-cover` before large PRs. Treat coverage as a diagnostic, not a gate.
- **No `testing.Short()` skips by default**. If a test must be slow or external, gate it on an explicit build tag (e.g. `//go:build integration`) and document how to run it.

## Linting

`golangci-lint` config will live in `.golangci.yml`. Suggested baseline linters: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gofmt`, `goimports`, `misspell`, `revive`, `gosec`, `bodyclose`, `errorlint`, `nolintlint`. Do not disable linters to silence findings — fix the code.

- **Fix the finding, don't suppress it.** `//nolint:<linter>` is only acceptable with a trailing comment explaining *why* the rule is wrong for this site (`nolintlint` enforces this).
- **Formatting is non-negotiable**: `goimports` with `-local github.com/helixml/comms-demo` groups local imports separately. Run `make fmt` before committing; CI runs `make fmt-check`.
- **Error wrapping**: `errorlint` enforces `%w` instead of `%v` for errors, and forbids type-assertion on errors — use `errors.As` / `errors.Is`.
- **`gosec`** flags raw SQL string concatenation, weak crypto, and command injection. The fix is almost always to restructure the code, not to suppress the warning.
- **New linters** are added in a dedicated `chore: enable <linter>` PR that also fixes all findings it surfaces — never bundled with unrelated changes.
