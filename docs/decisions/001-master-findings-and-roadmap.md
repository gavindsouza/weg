# ADR-001: Master Expert Panel Findings & Prioritized Roadmap

**Status:** Proposed
**Date:** 2026-02-09
**Decision Makers:** 9-persona expert panel (simulated)
**Compiled by:** Team Lead

---

## Executive Summary

Nine expert personas across three panels reviewed the weg project. The unanimous finding: **the product is technically strong but completely invisible**. The engineering quality (4ms startup, 7 deps, 92% error test coverage, proper file locking) is above-average for pre-release OSS. The distribution, marketing, and community infrastructure is at zero.

### Key Numbers

| Metric | Value | Source |
|--------|-------|--------|
| Startup time | 4ms | Panel 3 measurement (F6) |
| Binary size | 9.5MB stripped | Panel 3 measurement (F5) |
| Direct dependencies | 7 | go.mod (F7) |
| Commands | 70+ | Code exploration (F2) |
| GitHub stars | 1 | GitHub API (F10) |
| Releases | 0 | GitHub API (F10) |
| Test files | 31 | Code exploration |
| Critical findings | 7 | Panels 1-3 |
| High findings | 10 | Panels 1-3 |

---

## All Findings Consolidated (Sorted by Severity)

### Critical (7)

| ID | Finding | Panel | Personas | Evidence |
|----|---------|-------|----------|----------|
| DHH-1 | `addAppToWegToml` is a stub that tells user to edit config manually | P1 | DHH | `cmd/app/get.go:136-147` |
| AR-1 | MCP `weg_exec` allows argument injection via `strings.Fields()` | P1 | Armin | `cmd/mcp/handlers.go:127` |
| MH-1 | cmd/ tests are structural (registration), not behavioral | P2 | Mitchell H | `cmd/site/site_test.go`, `cmd/app/app_test.go` |
| MH-2 | No acceptance tests or golden file tests for CLI output | P2 | Mitchell H | Absence in test suite |
| PH-4 | No GitHub release — install instructions 404 | P3 | PostHog, Prime, Kelsey | GitHub API: 0 releases |
| PH-1 | README buries the value proposition below etymology | P3 | PostHog, Kelsey | `README.md:1-7` |
| PH-2 | No demo GIF/video in README | P3 | PostHog, Prime, Kelsey | README inspection |

### High (10)

| ID | Finding | Panel | Evidence |
|----|---------|-------|----------|
| LT-1 | Process-wide `os.Chdir()` in PersistentPreRunE | P1 | `cmd/root.go:91,123` |
| LT-2 | Global mutable state in output package (7 vars) | P1 | `internal/output/output.go:55-77` |
| DHH-2 | Two config formats with divergent schemas, no unified interface | P1 | `internal/config/wegtoml.go`, `pyproject.go` |
| DHH-3 | 70+ commands but critical ones are stubs | P1 | `cmd/site/site.go`, README vs reality |
| AR-2 | Error context lost in MCP handlers (stringified, loses type info) | P1 | `cmd/mcp/handlers.go` |
| GVR-1 | 5-context detection is implicit with no user visibility | P2 | `internal/config/detect.go:52-116` |
| GVR-2 | `api` command has two hidden code paths (local/remote) | P2 | `cmd/api/get.go:47-119` |
| MH-3 | Test helper `setupTestBench` needs to be shared across packages | P2 | `internal/completion/completion_test.go:153-190` |
| PH-3 | Zero GitHub repo metadata (no desc, topics, website) | P3 | GitHub API |
| PH-5 | No CONTRIBUTING.md, issue templates, or PR template | P3 | File absence |

### Medium (12)

| ID | Finding | Panel | Evidence |
|----|---------|-------|----------|
| LT-3 | `AppNames()`/`SiteNames()` claim sorted but iterate map randomly | P1 | `internal/state/state.go:278-294` |
| LT-5 | File lock failure silently falls back to unlocked reads | P1 | `internal/state/state.go:87-88` |
| LT-6 | 173-line Dockerfile embedded as Go string literal | P1 | `internal/container/image.go:41-214` |
| AR-3 | Secret redactor misses JWT, AWS, GitHub token patterns | P1 | `internal/output/redact.go` |
| AR-4 | Debug infrastructure exists but zero actual instrumentation | P1 | `internal/output/debug.go` vs cmd/ files |
| AR-5 | MCP `handleWegSiteList` loads state from wrong path for app mode | P1 | `cmd/mcp/handlers.go:298` |
| AR-6 | MCP handlers accept but ignore `context.Context` (no cancellation) | P1 | `cmd/mcp/handlers.go:18-33` |
| GVR-3 | `weg api get` and `weg doc get` do the same thing | P2 | README, `cmd/api/get.go`, `cmd/doc/` |
| GVR-4 | `skipAutoChdir` map is maintenance hazard | P2 | `cmd/root.go:97-109` |
| MH-6 | Output tests require 25+ save/restore boilerplate blocks | P2 | `internal/output/output_test.go` |
| MH-7 | No concurrent state access test despite `Flock` implementation | P2 | `internal/state/state_test.go` |
| MH-9 | `configureOutput` precedence logic not directly tested | P2 | `cmd/root.go:191-234` |

### Low (4)

| ID | Finding | Panel | Evidence |
|----|---------|-------|----------|
| LT-4 | Custom `indexOf` reimplements `strings.Index` worse | P1 | `cmd/version.go:138-144` |
| LT-7 | Podman-first container detection surprises Docker users | P1 | `internal/container/image.go:316-317` |
| GVR-6 | `api run` verb collides with `weg run` semantics | P2 | USAGE.md |
| DHH-7 | "weg" naming requires explanation for English speakers | P1 | README.md:7 |

### Positive (Strengths to Preserve)

| ID | Finding | Panel | Evidence |
|----|---------|-------|----------|
| DHH-4 | Excellent output format auto-detection (TTY/pipe) | P1 | `internal/output/output.go:167-175` |
| MH-4 | State management with proper locking and atomic writes | P2 | `internal/state/state.go` |
| MH-5 | Error type hierarchy is production-quality (92.9% coverage) | P2 | `internal/errors/errors.go` |
| SR-1 | README has excellent structure and examples | P2 | `README.md:1-344` |
| SR-2 | API command help text is gold standard for other commands | P2 | `cmd/api/api.go:27-51` |
| SR-4 | Output format system is auto-documenting | P2 | `internal/output/output.go` |
| MH-8 | Integration test structure is correct | P2 | `tests/integration_test.go` |
| TP-1 | 4ms startup is elite performance | P3 | Runtime measurement |
| TP-3 | 7 dependencies is disciplined | P3 | go.mod |
| KH-5 | Declarative config is the right paradigm shift | P3 | `weg.toml` design |

---

## Proposed ADRs (Ordered by Priority)

### P0: Do Before Any Public Launch

| # | ADR Title | Addresses | Scope |
|---|-----------|-----------|-------|
| ADR-002 | Cut v0.1.0 Release | PH-4, TP-4, KH-4 | CI/CD, git tags |
| ADR-003 | Restructure README for Discovery | PH-1, PH-2, KH-1 | README.md |
| ADR-004 | Set GitHub Repository Metadata | PH-3 | GitHub settings |
| ADR-005 | Fix MCP Argument Injection | AR-1 | `cmd/mcp/handlers.go` |

### P1: First 2 Weeks

| # | ADR Title | Addresses | Scope |
|---|-----------|-----------|-------|
| ADR-006 | Complete Config Write-Back (kill the stub) | DHH-1 | `cmd/app/get.go`, `internal/config/` |
| ADR-007 | Add Community Health Files | PH-5 | CONTRIBUTING.md, templates |
| ADR-008 | Adopt Behavioral Testing for CLI Commands | MH-1, MH-2, MH-3 | `internal/testutil/`, `cmd/` tests |
| ADR-009 | Consolidate CI Workflows | TP-6 | `.github/workflows/` |
| ADR-010 | MCP Security Hardening (structured errors, context propagation) | AR-2, AR-6 | `cmd/mcp/` |

### P2: First Month

| # | ADR Title | Addresses | Scope |
|---|-----------|-----------|-------|
| ADR-011 | Replace `os.Chdir()` with Explicit Path Threading | LT-1, DHH-6, GVR-4 | `cmd/root.go`, all commands |
| ADR-012 | Introduce OutputConfig Struct (replace globals) | LT-2, MH-6, GVR-5 | `internal/output/`, consumers |
| ADR-013 | Unified ProjectConfig Interface | DHH-2, AR-7 | `internal/config/` |
| ADR-014 | Make Context Detection Explicit and Visible | GVR-1, KH-3 | `weg status` enhancement |
| ADR-015 | Standardize Command Help Text Quality | SR-3, SR-7 | All cmd/ packages |
| ADR-016 | Expand Secret Redaction Patterns | AR-3 | `internal/output/redact.go` |
| ADR-017 | Embed Dockerfile Template via go:embed | LT-6 | `internal/container/` |
| ADR-018 | Consolidate Bench-Path Resolution | AR-7, F20 | `cmd/helpers.go`, consumers |

### P3: Ongoing

| # | ADR Title | Addresses | Scope |
|---|-----------|-----------|-------|
| ADR-019 | Wire Up Debug/Trace Infrastructure | AR-4 | All cmd/ packages |
| ADR-020 | Fix Unsorted "Sorted" Functions | LT-3 | `internal/state/state.go` |
| ADR-021 | Fix MCP State Loading Path Bug | AR-5 | `cmd/mcp/handlers.go` |

---

## Prioritized Roadmap

### Phase 0: "Ship It" (This Week)

**Goal:** Make weg installable and discoverable.

| Task | Addresses | Semantic Commit | TDD? |
|------|-----------|----------------|------|
| Fix `strings.Fields` argument injection in MCP | AR-1 | `fix(mcp): use proper argument tokenization for weg_exec` | Yes — test quoted args, special chars |
| Fix MCP state loading path bug | AR-5 | `fix(mcp): use benchPath for state.Load in app-centric mode` | Yes — test with app-centric fixture |
| Fix `AppNames()`/`SiteNames()` unsorted maps | LT-3 | `fix(state): sort AppNames and SiteNames return values` | Yes — test deterministic order |
| Replace custom `indexOf` with `strings.Index` | LT-4 | `refactor(version): use strings.Index instead of custom indexOf` | Existing tests cover |
| Delete duplicate `ci-cd.yml` workflow | TP-6 | `chore(ci): remove duplicate ci-cd workflow` | N/A |
| Set GitHub metadata (description, topics) | PH-3 | N/A (GitHub UI) | N/A |
| Tag v0.1.0 release | PH-4 | `chore: tag v0.1.0 initial release` | Verify binaries download |
| Verify install instructions work post-release | PH-4 | `fix(docs): update install URLs if needed` | Manual verification |

### Phase 1: "Look Professional" (Week 2)

**Goal:** README sells the product, community can contribute.

| Task | Addresses | Semantic Commit | TDD? |
|------|-----------|----------------|------|
| Record demo GIF with vhs/asciinema | PH-2 | `docs: add terminal demo recording` | Manual |
| Restructure README (landing page style) | PH-1, KH-1 | `docs: restructure README as landing page` | N/A |
| Add badges (CI, release, license, Go report) | PH-7 | Part of README restructure | N/A |
| Add performance numbers to README | TP-1 | Part of README restructure | N/A |
| Move comparison table to top of README | PH-7 | Part of README restructure | N/A |
| Create CONTRIBUTING.md | PH-5 | `docs: add CONTRIBUTING.md` | N/A |
| Create issue templates (.github/ISSUE_TEMPLATE/) | PH-5 | `chore: add issue templates` | N/A |
| Create PR template | PH-5 | `chore: add pull request template` | N/A |
| Create CHANGELOG.md | PH-6 | `docs: add CHANGELOG.md starting from v0.1.0` | N/A |
| Add `go install` to README | KH-4 | Part of README restructure | N/A |

### Phase 2: "Test Everything" (Weeks 2-4)

**Goal:** Behavioral test coverage for cmd/ layer, shared test infrastructure.

| Task | Addresses | Semantic Commit | TDD? |
|------|-----------|----------------|------|
| Create `internal/testutil` package | MH-3, MH-6 | `feat(testutil): add shared bench fixture builder and output capture` | Yes |
| Add `testutil.NewBench(t)` builder | MH-3 | Part of testutil | Yes |
| Add `testutil.CaptureOutput(t)` | MH-6 | Part of testutil | Yes |
| Add `testutil.RunCommand(t, args)` | MH-2 | Part of testutil | Yes |
| Add behavioral tests for `weg site list` | MH-1 | `test(site): add behavioral tests for site list command` | TDD |
| Add behavioral tests for `weg site new` | MH-1 | `test(site): add behavioral tests for site new command` | TDD |
| Add behavioral tests for `weg app list` | MH-1 | `test(app): add behavioral tests for app list command` | TDD |
| Add behavioral tests for `weg status` | MH-1, GVR-1 | `test(cmd): add behavioral tests for status command` | TDD |
| Add behavioral tests for `weg config show` | MH-1 | `test(config): add behavioral tests for config show` | TDD |
| Add `configureOutput` precedence tests | MH-9 | `test(cmd): add output configuration precedence tests` | TDD |
| Add concurrent state access test | MH-7 | `test(state): add concurrent read/write test with Flock` | TDD |
| Golden file tests for help output | MH-2 | `test(cmd): add golden file tests for help text` | TDD |

### Phase 3: "Harden" (Month 2)

**Goal:** Fix architectural issues identified by the panel.

| Task | Addresses | Semantic Commit | TDD? |
|------|-----------|----------------|------|
| Complete `addAppToWegToml` implementation | DHH-1 | `feat(app): implement TOML write-back for weg app get` | TDD |
| Use `exec.CommandContext` in MCP handlers | AR-6 | `fix(mcp): propagate context for subprocess cancellation` | Yes |
| Return structured errors from MCP handlers | AR-2 | `feat(mcp): return structured error responses with codes` | TDD |
| Expand redaction patterns (JWT, AWS, GH tokens) | AR-3 | `feat(output): expand secret redaction to modern token formats` | TDD |
| Embed Dockerfile template with go:embed | LT-6 | `refactor(container): embed Dockerfile template` | Existing tests |
| Consolidate bench-path resolution | AR-7 | `refactor: consolidate bench-path resolution into single utility` | Yes |
| Wire up debug instrumentation in cmd/ | AR-4 | `feat(cmd): add debug/trace instrumentation to commands` | Manual verification |
| Standardize help text across all commands | SR-3 | `docs(cmd): standardize Long descriptions and Examples` | Golden file tests |

### Phase 4: "Architect" (Month 3)

**Goal:** Address the deep architectural findings.

| Task | Addresses | Semantic Commit | TDD? |
|------|-----------|----------------|------|
| Replace `os.Chdir()` with path threading | LT-1, GVR-4 | `refactor(cmd): replace os.Chdir with explicit path parameter` | Yes — test path isolation |
| Introduce `OutputConfig` struct | LT-2, MH-6 | `refactor(output): introduce OutputConfig struct for dependency injection` | Yes — test parallel output |
| Create unified `ProjectConfig` interface | DHH-2 | `refactor(config): unified ProjectConfig interface for weg.toml and pyproject.toml` | TDD |
| Enhance `weg status` with context visibility | GVR-1 | `feat(cmd): add context visibility to weg status` | TDD |
| Add `WEG_CONTEXT` env var override | DHH-5 | `feat(config): add WEG_CONTEXT environment variable override` | TDD |

---

## Semantic Commit Convention

Based on findings, weg should adopt [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

Types: feat, fix, refactor, test, docs, chore, perf, ci
Scopes: cmd, mcp, output, state, config, container, api, site, app, testutil, ci
```

**References:**
- Angular commit convention — *Source: https://github.com/angular/angular/blob/main/CONTRIBUTING.md#commit*
- Conventional Commits spec — *Source: https://www.conventionalcommits.org/en/v1.0.0/*

---

## TDD Protocol

For every code change:

1. **Write the failing test first** — describe the expected behavior
2. **Run the test** — confirm it fails for the right reason
3. **Implement the minimum code** to pass
4. **Refactor** — clean up while tests stay green
5. **Commit** — semantic commit with test and implementation together

**Test types by priority:**
1. Behavioral tests (cmd/ commands with fixtures) — most value
2. Unit tests (internal/ packages) — well-covered already
3. Integration tests (cross-package workflows) — good existing foundation
4. Golden file tests (output formatting) — catch regressions

---

## Cross-References

| Panel | Document | Key Findings |
|-------|----------|--------------|
| Panel 1: Architecture & DX | `docs/reviews/panel-1-architecture-dx.md` | LT-1 through LT-7, DHH-1 through DHH-7, AR-1 through AR-7 |
| Panel 2: API & Testing | `docs/reviews/panel-2-api-testing.md` | SR-1 through SR-7, GVR-1 through GVR-7, MH-1 through MH-9 |
| Panel 3: OSS Growth | `docs/reviews/panel-3-oss-growth.md` | PH-1 through PH-8, TP-1 through TP-8, KH-1 through KH-7 |

---

*Total findings: 33 (7 critical, 10 high, 12 medium, 4 low) + 10 positive strengths*
*Total proposed ADRs: 21*
*Estimated timeline: 3-4 months to complete all phases*
