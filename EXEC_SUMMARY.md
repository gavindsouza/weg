# Weg: Executive Summary

## What is Weg?

A modern replacement for Frappe's `bench` CLI. Faster, declarative, developer-friendly.

## Why Build This?

| Problem with Bench | Weg Solution |
|-------------------|--------------|
| Imperative commands, state drift | Declarative `weg.toml` config |
| Slow pip installs | 10-100x faster with `uv` |
| Complex multi-Python setup | Nix-based `devbox` isolation |
| App buried in bench structure | App-centric: your code is the root |
| No direct API access | `weg api` - skip HTTP entirely |

## Current Status

```
██████████████████░░░░░░░░ 72% Feature Complete
████████░░░░░░░░░░░░░░░░░░ 32% Test Coverage
██████████████████████░░░░ 88% Core Workflows
```

## Blockers to Launch

### Must Fix (P0)

| Gap | Impact | Effort |
|-----|--------|--------|
| No backup/restore | Developers can't safely experiment | 3 days |
| No cache clear | #1 support question for Frappe devs | 0.5 day |
| No password reset | Broken workflow after restore | 0.5 day |
| 80% code untested | Risk of regressions, bugs | 5 days |

### Should Fix (P1)

| Gap | Impact | Effort |
|-----|--------|--------|
| No scheduler control | Can't test background jobs | 1 day |
| No job visibility | Can't debug failed tasks | 1 day |
| No fixture export | Can't version control data | 2 days |
| Data corruption possible | Config loss on crash | 1 day |

## Timeline to Market-Ready

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| 0: Foundation | 2 weeks | Tests, atomic writes, bug fixes |
| 1: Essential Commands | 2 weeks | backup, restore, cache, password |
| 2: Scheduler | 1 week | enable, disable, jobs |
| 3: Data Tools | 1 week | fixtures, import/export |
| 4: Polish | 1 week | hosts, reinstall, shell |
| 5: Docs & QA | 1 week | 80% coverage, full docs |

**Total: 8 weeks to launch-ready**

## Competitive Position

| Feature | Bench | Weg |
|---------|-------|-----|
| Install speed | Slow (pip) | Fast (uv) |
| Environment isolation | virtualenv | Nix (devbox) |
| Configuration | Imperative | Declarative |
| Multi-version dev | Manual | Built-in |
| Cloud deploy | Separate tool | Integrated |
| Container builds | frappe_docker | Built-in |
| Direct API access | No | Yes |

## Resource Ask

- **Engineering**: 1 senior dev, 8 weeks focused
- **Testing**: Access to Frappe 14, 15, 16 environments
- **Documentation**: Technical writer for final polish

## Success Criteria

1. Zero fallback to `bench` for common workflows
2. 80%+ test coverage
3. <5 min from clone to running site
4. All errors have actionable suggestions

## Risks

| Risk | Mitigation |
|------|------------|
| Frappe API changes | Pin versions, test matrix |
| devbox learning curve | Clear docs, `weg self install-tools` |
| Adoption resistance | Benchmark comparisons, migration guide |

---

**Recommendation**: Proceed with Phase 0 immediately. The foundation work de-risks everything else.
