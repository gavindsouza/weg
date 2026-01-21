# Miner Polecat

The Miner is a specialized polecat for discovering developer pain points from external sources. It feeds the R&D pipeline by maintaining a problem inventory.

## Purpose

In the R&D workflow:
- **Understanding problems: 45%** ← Miner's domain
- Designing solutions: 52%
- Implementation: 3%

The Miner automates the discovery phase by systematically scraping forums, GitHub issues, and other sources to identify recurring problems that developers face with Frappe/bench.

## Quick Start

### 1. Create a Mining Task

```bash
# Create the tracking issue
bd create --type=task --labels="mining,research" \
  --title="Mine Frappe Forum for installation issues" \
  --description="Focus: Installation and setup pain points
Source: Frappe Forum (discuss.frappe.io)
Query: bench init, installation error
Timeframe: 30d
Max items: 50"
```

### 2. Dispatch a Miner Polecat

```bash
# Sling work to a polecat with the miner formula
gt sling <issue-id> --formula=mol-polecat-miner \
  --vars="source=frappe_forum,query=installation,timeframe=30d,max_items=50"
```

### 3. Monitor Progress

```bash
# Check polecat status
gt witness status

# View filed problem beads
bd list --labels=problem,mined
```

## Mining Sources

See `mining-sources.yaml` for the full configuration. Key sources:

| Source | Type | Signal Quality |
|--------|------|----------------|
| Frappe Forum | Discourse | High - direct user complaints |
| GitHub frappe/bench | Issues | High - specific bugs/requests |
| GitHub frappe/frappe | Issues | Medium - broader scope |
| Reddit r/erpnext | Reddit | Medium - honest feedback |
| Stack Overflow | Q&A | Medium - technical issues |

## Problem Bead Schema

Mined problems are filed as beads with this structure:

```yaml
type: task
labels: [problem, <category>, mined]
priority: 1-4 (based on score)
title: "Problem: <clear description>"
description: |
  ## Problem Statement
  <what the pain point is>

  ## Evidence
  - Source: <url>
  - Frequency: <how often mentioned>
  - User quotes:
    > <direct quotes>

  ## Impact
  - Severity: X/10
  - User type: beginner/experienced/enterprise
  - Context: install/development/production

  ## Score
  - Mining score: XX/100
  - Breakdown: Freq(X*3) + Sev(X*3) + Addr(X*2) + Align(X*2)

  ## Workarounds
  <known workarounds or "None known">
```

## Scoring System

Problems are scored 1-100 based on four factors:

| Factor | Weight | Description |
|--------|--------|-------------|
| Frequency | 3x | How often is this mentioned? |
| Severity | 3x | How painful? (blocking=10, annoying=3) |
| Addressable | 2x | Can weg realistically solve this? |
| Alignment | 2x | Does it fit weg's mission? |

**Score = (Freq × 3) + (Severity × 3) + (Addressable × 2) + (Alignment × 2)**

### Priority Mapping

| Score | Priority | Meaning |
|-------|----------|---------|
| 80-100 | P1 | Critical pain - address immediately |
| 60-79 | P2 | Significant pain - plan for soon |
| 40-59 | P3 | Moderate pain - consider for roadmap |
| 1-39 | P4 | Minor pain - nice to have |

## Categories

Problems are categorized for routing:

| Category | Description | Example |
|----------|-------------|---------|
| `installation` | Setup, dependencies, environment | "bench init fails on Mac M1" |
| `cli-ux` | Command usability, error messages | "Cryptic error from bench migrate" |
| `performance` | Speed, memory, scaling | "bench start takes 30 seconds" |
| `workflow` | Developer workflow friction | "Can't test without full bench" |
| `documentation` | Missing or unclear docs | "No examples for custom reports" |
| `customization` | DocTypes, scripts, reports | "Client script debugging impossible" |
| `remote-dev` | Remote site development | "Can't develop on Frappe Cloud site" |
| `integration` | Third-party integrations | "Webhook reliability issues" |
| `debugging` | Troubleshooting difficulties | "Stack traces unhelpful" |

## Suggested Mining Campaigns

### Weekly Sweep
```bash
# Monitor active pain points
bd create --type=task --labels="mining,weekly" \
  --title="Weekly mining sweep - $(date +%Y-%m-%d)" \
  --description="Sources: frappe_forum, github_bench
Queries: installation, cli_usability
Timeframe: 7d
Max: 50"
```

### Deep Dive: Remote Development
```bash
# Focus on remote site development pain
bd create --type=task --labels="mining,deep-dive" \
  --title="Deep dive: Remote development pain points" \
  --description="Sources: frappe_forum, github_bench, stackoverflow
Queries: remote site, frappe cloud, sync customizations, deploy
Timeframe: 90d
Max: 100"
```

### Competitive Analysis
```bash
# What do people wish bench did better?
bd create --type=task --labels="mining,competitive" \
  --title="Mining: Bench wishlist and comparisons" \
  --description="Sources: frappe_forum, reddit
Queries: bench alternative, wish bench could, bench vs
Timeframe: all
Max: 50"
```

## Problem Inventory Maintenance

### View Current Inventory
```bash
# All problems
bd list --labels=problem

# By category
bd list --labels=problem,installation
bd list --labels=problem,cli-ux

# High priority only
bd list --labels=problem --priority=1,2

# With scores
bd list --labels=problem --sort=priority
```

### Deduplicate
When mining finds a duplicate:
```bash
# Add evidence to existing bead
bd update <existing-id> --notes "Additional evidence from mining..."

# Link related beads
bd link <new-id> <existing-id>
```

### Promote to Spec
When a problem is ready for solution design:
```bash
# Create spec issue referencing problem
bd create --type=task --labels="spec,design" \
  --title="Spec: Solution for <problem>" \
  --description="Problem bead: <problem-id>
..."

# Link them
bd link <spec-id> <problem-id>
```

## Integration with R&D Pipeline

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   MINER     │────▶│  ARBITER    │────▶│   SMITH     │
│  Discovery  │     │  Decision   │     │   Build     │
│             │     │             │     │             │
│ Files:      │     │ Reviews:    │     │ Implements: │
│ - problems  │     │ - problems  │     │ - specs     │
│ - evidence  │     │ - solutions │     │ - code      │
│ - scores    │     │ - taste     │     │ - tests     │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │                   │
       ▼                   ▼                   ▼
   Problem            Solution              Feature
   Inventory          Landscape             Shipped
```

## Tips for Effective Mining

1. **Be specific with queries** - "bench init error python" > "bench problem"
2. **Look for patterns** - Multiple similar complaints = real problem
3. **Capture quotes** - User words are powerful evidence
4. **Note workarounds** - Workarounds indicate unmet needs
5. **Track frequency** - "Everyone has this" vs "rare edge case"
6. **Consider user type** - Beginner confusion vs expert limitation
7. **Check recency** - Old issues may be fixed; check versions
