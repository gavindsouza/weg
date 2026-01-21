# Cartographer Polecat

The Cartographer maps solution spaces for discovered problems. It takes a problem brief and systematically explores ALL viable solutions, producing a landscape document that feeds into Arbiter decision-making.

## Purpose

In the R&D workflow:
- Understanding problems: 45% (Miner)
- **Designing solutions: 52%** ← Cartographer's domain (solution mapping)
- Implementation: 3% (Smith)

The Cartographer bridges discovery and implementation by turning vague problems into concrete, comparable options.

## Quick Start

### 1. Create a Mapping Task

```bash
# Create tracking issue referencing a problem bead
bd create --type=task --labels="mapping,design" \
  --title="Map solutions for: Cannot develop on Frappe Cloud" \
  --description="Problem bead: <problem-id>
Map all viable solutions for enabling remote site development.

Constraints:
- Must work without bench access
- Should integrate with git
- Needs to handle conflicts"
```

### 2. Dispatch a Cartographer Polecat

```bash
gt sling <issue-id> --formula=mol-polecat-cartographer \
  --vars="problem=<problem-bead-id>,constraints=no-bench-access"
```

### 3. Review the Landscape

```bash
# Find the landscape document
ls research/landscapes/

# View the landscape bead
bd show <landscape-bead-id>
```

## Cartographer Formula Steps

1. **load-context** - Deeply understand the problem
2. **research-prior-art** - What exists? What can we learn?
3. **brainstorm-solutions** - Generate ALL viable options (5-15)
4. **evaluate-solutions** - Score and filter to top 3-5
5. **deep-dive-analysis** - Detailed pros/cons/effort for each
6. **synthesize-recommendation** - Rank and recommend
7. **write-landscape-document** - Produce formal document
8. **file-landscape-bead** - Create bead, link to problem
9. **complete-and-exit** - Commit and self-clean

## Solution Landscape Template

```markdown
# Solution Landscape: <Problem Title>

**Problem:** <bead-id>
**Date:** <date>
**Cartographer:** <who>

## Problem Summary
<from problem bead>

## Constraints
- <constraint 1>
- <constraint 2>

## Prior Art

### <Prior Art 1>
- What it is
- Good: ...
- Bad: ...
- Lesson: ...

## Solutions Evaluated

### Option A: <Name> (Recommended)
**Description:** How it works
**User Experience:** Step by step
**Pros:** ...
**Cons:** ...
**Effort:** S/M/L/XL

### Option B: <Name>
...

## Comparison Matrix

| Criterion | Weight | Option A | Option B | Option C |
|-----------|--------|----------|----------|----------|
| Effectiveness | 3x | 8 | 9 | 6 |
| Simplicity | 2x | 7 | 5 | 9 |
| Feasibility | 2x | 6 | 4 | 9 |
| Total | - | 71 | 64 | 73 |

## Recommendation

**Recommended:** Option A

**Rationale:** ...

**Conditions for Option B:** ...

**Uncertainties:** ...

## Next Steps
1. ...
2. ...
```

## Evaluation Criteria

| Criterion | Weight | Description |
|-----------|--------|-------------|
| Effectiveness | 3x | How well does it solve the problem? |
| Simplicity | 2x | How easy to understand and use? |
| Feasibility | 2x | How hard to build? |
| Maintainability | 1x | Long-term maintenance burden? |
| Compatibility | 1x | Works with existing ecosystem? |
| Extensibility | 1x | Can it grow with future needs? |

**Score each 1-10, multiply by weight, sum for total.**

## Brainstorming Techniques

1. **Obvious** - What's the straightforward approach?
2. **Prior art** - What can we borrow/adapt?
3. **Opposite** - What if we did the inverse?
4. **Minimalist** - Simplest possible solution?
5. **Maximalist** - Most powerful solution?
6. **User-driven** - What would users design?
7. **Constraint removal** - What if X didn't apply?
8. **Combination** - Can we merge approaches?

## Output Artifacts

1. **Landscape Document** - `research/landscapes/<problem-id>-landscape.md`
2. **Landscape Bead** - Links to problem, ready for Arbiter
3. **Updated Problem Bead** - Notes pointing to landscape

## Integration with Pipeline

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│   MINER     │────▶│   CARTOGRAPHER   │────▶│   ARBITER   │
│             │     │                  │     │             │
│ Problem:    │     │ Landscape:       │     │ Decision:   │
│ "Can't dev  │     │ - Option A: ...  │     │ "Go with    │
│  on Cloud"  │     │ - Option B: ...  │     │  Option A"  │
│             │     │ - Recommend: A   │     │             │
└─────────────┘     └──────────────────┘     └─────────────┘
```

## Tips for Effective Mapping

1. **Don't filter too early** - Capture all ideas first
2. **Steel-man each option** - Present each at its best
3. **Be honest about trade-offs** - No option is perfect
4. **Research deeply** - Prior art reveals hidden insights
5. **Think about users** - Who will use this? How?
6. **Consider maintenance** - Who maintains this long-term?
7. **Flag uncertainties** - What assumptions are you making?

## Example: Remote Development Problem

**Problem:** Cannot develop on Frappe Cloud

**Options Mapped:**
1. **weg clone** - Git-backed local sync via REST API
2. **Browser IDE** - VS Code in browser with API access
3. **SSH tunnel** - Direct access via SSH forwarding
4. **Frappe Cloud dev mode** - Request feature from FC
5. **Local proxy** - Intercept and redirect API calls

**Evaluation:**
- Option 1 scored highest (effectiveness + feasibility)
- Option 2 interesting but complex infrastructure
- Options 3-5 depend on external parties

**Recommendation:** Option 1 (weg clone)
