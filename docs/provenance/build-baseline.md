# Build baseline on the untouched 2024 dependency set

Recorded 2026-09-06 at commit `7482766` (tree identical to fork point `62bde16` apart from
docs and governance files). `go.mod`/`go.sum` unmodified: `go 1.21`, cosmos-sdk v0.45.16,
tendermint v0.34.27 (→ cometbft), sentinel-official/hub v0.11.3. `go mod tidy` was **not**
run, so these results describe exactly what upstream shipped.

## Period-correct toolchain: Go 1.21.13

| Step | Result |
|---|---|
| `GOTOOLCHAIN=go1.21.13 go mod verify` | `all modules verified` |
| `GOTOOLCHAIN=go1.21.13 go build ./...` | exit 0 |
| `GOTOOLCHAIN=go1.21.13 go vet ./...` | exit 0, no findings |
| `GOTOOLCHAIN=go1.21.13 make build` | exit 0 → `bin/sentinelnode`, 40,780,896 bytes, `-tags netgo -trimpath`, cgo (sqlite) |

Quirk: the Makefile derives `VERSION` from `git describe --tags`, which now resolves to the
annotated `fork-point` tag and yields `fork-point-2`. Harmless for the baseline; the
Makefile is rewritten in the rebrand.

## Stock local toolchain: Go 1.27.0

`go build ./...` exit 0, `go vet ./...` exit 0 — the 2024 dependency set compiles unchanged
on a 2026 toolchain. No toolchain blocker exists for
the modernisation phase; the work there is protocol drift (v2 → v3 chain messages) and
dependency upgrades, not compiler compatibility.

## Not exercised locally

`make build-image` and `scripts/runner.sh` need Docker, which is not installed on the build
host. `make test` runs but upstream ships no meaningful tests.
