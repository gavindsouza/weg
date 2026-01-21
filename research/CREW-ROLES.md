# R&D Crew Roles

Specialized roles for the weg R&D pipeline. Each role has a specific focus area in the discovery → design → implementation flow.

## Role Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           R&D PIPELINE                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   DISCOVER (45%)              DESIGN (52%)              IMPLEMENT (3%)      │
│                                                                             │
│   ┌─────────┐                ┌─────────────┐           ┌─────────┐         │
│   │  MINER  │───────────────▶│ CARTOGRAPHER│──────────▶│  SMITH  │         │
│   │         │                │             │           │         │         │
│   │ Finds   │                │ Maps        │           │ Builds  │         │
│   │ problems│                │ solutions   │           │ code    │         │
│   └─────────┘                └──────┬──────┘           └────┬────┘         │
│                                     │                       │              │
│                              ┌──────▼──────┐          ┌─────▼─────┐        │
│                              │   ARBITER   │          │  SCRIBE   │        │
│                              │             │          │           │        │
│                              │ Decides     │          │ Documents │        │
│                              │ (taste)     │          │           │        │
│                              └──────┬──────┘          └───────────┘        │
│                                     │                                      │
│                              ┌──────▼──────┐                               │
│                              │  ADVERSARY  │                               │
│                              │             │                               │
│                              │ Challenges  │                               │
│                              │ proposals   │                               │
│                              └─────────────┘                               │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Role Definitions

### Miner (Discovery Polecat)

**Formula:** `mol-polecat-miner`
**Status:** ✅ Implemented

**Focus:** Problem discovery and evidence gathering

**Inputs:**
- Mining source (forum, GitHub, reddit, etc.)
- Query/topic to focus on
- Timeframe

**Outputs:**
- Problem beads with evidence, scores, and categories
- Pattern observations
- Recommendations for deeper mining

**Key Responsibilities:**
- Scrape forums, GitHub issues, support channels
- Extract pain points with evidence
- Score problems by frequency, severity, addressability
- Categorize for routing
- Avoid duplicates, enrich existing beads

---

### Cartographer (Design Polecat)

**Formula:** `mol-polecat-cartographer` (TODO)
**Status:** 🔲 Not yet implemented

**Focus:** Solution space mapping

**Inputs:**
- Problem bead(s) to map
- Constraints and context

**Outputs:**
- Solution landscape document
- Trade-off analysis
- Prior art research
- Recommendation ranking

**Key Responsibilities:**
- Take problem brief → map ALL possible solutions
- Research prior art (what do competitors do?)
- Document trade-offs (speed vs simplicity vs power)
- Identify technical constraints
- Produce solution landscape doc

**Template Output:**
```markdown
# Solution Landscape: <Problem>

## Problem Summary
<from problem bead>

## Solution Options

### Option A: <Name>
- Description: ...
- Pros: ...
- Cons: ...
- Effort: S/M/L
- Prior art: ...

### Option B: <Name>
...

## Trade-off Matrix
| Option | Speed | Simplicity | Power | Effort |
|--------|-------|------------|-------|--------|
| A      | ++    | +          | -     | M      |
| B      | +     | ++         | +     | L      |

## Recommendation
<ranked list with rationale>
```

---

### Arbiter (Taste Curator - Senior Crew)

**Formula:** N/A - This is a human or senior crew role, not a polecat
**Status:** 🟡 Human role

**Focus:** Solution selection and taste curation

**Inputs:**
- Problem inventory (from Miner)
- Solution landscapes (from Cartographer)
- Context on weg mission and priorities

**Outputs:**
- Selected solutions with rationale
- Design decisions
- Spec assignments

**Key Responsibilities:**
- Review problem inventory regularly
- Apply taste criteria (speed, simplicity, power)
- Select which solutions to pursue
- Make design decisions with rationale
- Assign spec writing to appropriate crew/polecats

**Taste Criteria:**
1. **Speed** - Does this make developers faster?
2. **Simplicity** - Is the UX obvious and minimal?
3. **Power** - Does this unlock new capabilities?
4. **Alignment** - Does it fit weg's mission?
5. **Effort** - Is the ROI justified?

---

### Adversary (Challenge Polecat)

**Formula:** `mol-polecat-adversary` (TODO)
**Status:** 🔲 Not yet implemented

**Focus:** Stress-testing proposed solutions

**Inputs:**
- Proposed solution or spec
- Problem context

**Outputs:**
- Challenge report
- Edge cases and failure modes
- Alternative proposals
- Improved design (if issues found)

**Key Responsibilities:**
- Play devil's advocate
- Find edge cases and failure modes
- Propose alternatives that might be better
- Identify missing requirements
- Flag security/performance concerns

**Template Output:**
```markdown
# Challenge Report: <Spec/Solution>

## Summary
<brief description of what was challenged>

## Issues Found

### Critical
- <issue>: <why it's critical>

### Concerns
- <concern>: <potential impact>

## Edge Cases
- <case>: <how it breaks>

## Alternative Proposals
### Alternative A
<might be better because...>

## Recommendations
<accept as-is / revise / reconsider / reject>
```

---

### Smith (Implementation Polecat)

**Formula:** `mol-polecat-work` (existing)
**Status:** ✅ Implemented (as general polecat)

**Focus:** Rapid spec-to-code execution

**Inputs:**
- Spec issue with clear requirements
- Test criteria

**Outputs:**
- Working code
- Tests
- PR ready for review

**Key Responsibilities:**
- Implement from finalized spec
- Write tests alongside code
- Follow project conventions
- Submit for code review
- Fix review feedback

---

### Scribe (Documentation Polecat)

**Formula:** `mol-polecat-scribe` (TODO)
**Status:** 🔲 Not yet implemented

**Focus:** User-facing documentation

**Inputs:**
- Shipped feature
- Spec/design docs
- Code to document

**Outputs:**
- User documentation
- Examples and tutorials
- CLAUDE.md updates
- API documentation

**Key Responsibilities:**
- Take shipped features → write docs
- Create examples and tutorials
- Update CLAUDE.md guidelines
- Maintain command reference
- Write migration guides

---

## Workflow Examples

### New Problem → Solution → Feature

```
1. Mayor creates mining task for "remote development pain"
2. Miner polecat runs, files 15 problem beads
3. Arbiter (human) reviews, selects top 3 for solution mapping
4. Cartographer polecat maps solutions for each
5. Arbiter selects "weg clone" approach
6. Adversary polecat challenges the design
7. Arbiter finalizes spec (002-remote-clone)
8. Smith polecat implements
9. Code review polecat reviews
10. Scribe polecat documents
11. Feature shipped!
```

### Continuous Mining

```
Weekly:
1. Witness spawns Miner with weekly config
2. Miner scrapes, files problems, self-cleans
3. Problem inventory grows
4. Arbiter reviews periodically, promotes to specs

Ad-hoc:
1. Human notices pain point, creates mining task
2. Miner deep-dives on specific topic
3. Results feed into R&D pipeline
```

## Implementation Priority

| Role | Priority | Rationale |
|------|----------|-----------|
| Miner | ✅ Done | Foundation - feeds pipeline |
| Smith | ✅ Done | Existing polecat-work formula |
| Cartographer | P1 | Needed to map solutions |
| Adversary | P2 | Improves design quality |
| Scribe | P2 | Needed for docs |
| Arbiter | N/A | Human role |
