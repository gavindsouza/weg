# Mining Results: Remote Development Pain Points

**Date:** 2026-01-17
**Miner:** Mayor (test run)
**Source:** Frappe Forum (discuss.frappe.io)
**Query:** remote development, frappe cloud, customization sync, client script version control
**Timeframe:** All time (historical analysis)
**Items Analyzed:** 15+ threads

---

## Executive Summary

Remote development is a **major pain point** for Frappe developers. The recurring themes are:

1. **No development path on Frappe Cloud** - Users can't develop directly, must use local + deploy
2. **Version control of customizations is hard** - Client scripts, custom fields trapped in database
3. **Team collaboration on customizations is broken** - No good way to sync changes
4. **The workaround (custom apps) is heavyweight** - Overkill for simple customizations

This strongly validates the `weg clone` (002-remote-clone) spec.

---

## Problem Inventory

### Problem 1: Cannot Develop on Frappe Cloud

**Score: 85/100** (P1 - Critical)

| Factor | Score | Notes |
|--------|-------|-------|
| Frequency | 9 | Multiple threads, years of complaints |
| Severity | 9 | Blocks entire workflow |
| Addressable | 8 | weg clone directly solves this |
| Alignment | 10 | Core weg mission |

**Evidence:**

> "FC is not for development at least for now" - [Using Frappe Cloud for development](https://discuss.frappe.io/t/using-frappe-cloud-for-development/95397)

> "Developer mode doesn't work on shared benches. Users must migrate to a private bench first" - [How to enable developer mode in frappe cloud](https://discuss.frappe.io/t/how-to-enable-developer-mode-in-frappe-cloud/124602)

> "Users cannot make customizations directly on Frappe Cloud. Instead, they must create custom apps locally and deploy them" - Forum consensus

**Impact:**
- Affects: All Frappe Cloud users who want to customize
- Severity: Blocking - no workaround within Cloud
- Context: Any customization beyond basic settings

**Workarounds:**
- Develop locally, deploy via custom app (heavyweight)
- Use private bench + developer mode (extra cost, complexity)

---

### Problem 2: Version Control of Customizations is Broken

**Score: 82/100** (P1 - Critical)

| Factor | Score | Notes |
|--------|-------|-------|
| Frequency | 9 | Core recurring theme |
| Severity | 8 | Risk of lost work |
| Addressable | 9 | weg clone + git solves this |
| Alignment | 10 | Core weg mission |

**Evidence:**

> "Developers struggle with managing 'custom scripts, changing UI forms and fields' across multiple team members while maintaining synchronization" - [How to manage erpnext customizations](https://discuss.frappe.io/t/how-to-manage-erpnext-customizations-version-control/25008)

> "Developers worry that modifications to core doctypes will be lost during ERPNext updates" - [Best way to manage client scripts](https://discuss.frappe.io/t/best-way-to-manage-client-scripts-on-core-doctypes/99967)

> "The fundamental tension is balancing customizations with staying current—developers cannot easily maintain local modifications while 'upgrade to latest versions'" - Forum discussion

**Impact:**
- Affects: Any team with >1 developer
- Severity: High - data loss risk, sync conflicts
- Context: Ongoing development, updates

**Workarounds:**
- Export fixtures manually (tedious, error-prone)
- Create custom app for everything (overkill)

---

### Problem 3: Client Scripts Not Portable

**Score: 75/100** (P2 - Significant)

| Factor | Score | Notes |
|--------|-------|-------|
| Frequency | 8 | Common complaint |
| Severity | 7 | Painful but survivable |
| Addressable | 9 | weg clone handles this |
| Alignment | 9 | Fits remote-dev focus |

**Evidence:**

> "Version Control Challenges - Developers struggle with managing custom JavaScript scripts created through the UI alongside Python server-side code" - [Best practice for custom scripts](https://discuss.frappe.io/t/what-is-the-best-practice-for-custom-scripts/11983)

> "Deployment Consistency - Difficulty ensuring custom scripts transfer correctly between development and client environments" - Forum discussion

> "Scripts may not persist properly after migration, causing DocType reference failures" - Forum reports

**Impact:**
- Affects: Developers using client/server scripts
- Severity: Medium - workarounds exist but painful
- Context: Deployment, migration

**Workarounds:**
- bench export-fixtures (manual)
- Create app with fixtures/ directory
- Manual copy/paste (error-prone)

---

### Problem 4: Custom App Overhead for Simple Customizations

**Score: 70/100** (P2 - Significant)

| Factor | Score | Notes |
|--------|-------|-------|
| Frequency | 7 | Implicit in many threads |
| Severity | 6 | Friction, not blocking |
| Addressable | 9 | weg clone provides lightweight alternative |
| Alignment | 9 | Simplicity goal |

**Evidence:**

> "The consensus recommendation is to package custom functionality within a dedicated Frappe app" - Forum advice

> But creating an app requires: pyproject.toml, hooks.py, proper structure, bench get-app, install...

**Impact:**
- Affects: Developers with simple customizations
- Severity: Friction - overkill for small changes
- Context: Quick customizations, prototyping

**Workarounds:**
- Create full app anyway (heavy)
- Use fixtures without app (incomplete)
- Manual management (fragile)

---

### Problem 5: Developer Mode Confusion

**Score: 58/100** (P3 - Moderate)

| Factor | Score | Notes |
|--------|-------|-------|
| Frequency | 7 | Many threads about this |
| Severity | 5 | Confusing but solvable |
| Addressable | 6 | Partially addressed by weg |
| Alignment | 7 | DX improvement |

**Evidence:**

> "With so much of headache i installed erpnext 13... I was trying to make in production level but could not" - [Developer mode to Production mode](https://discuss.frappe.io/t/developer-mode-to-production-mode-in-erpnext-13/79412)

> "A user reported getting the error 'Not in Developer Mode' even though they had already added developer_mode: 1" - [Set Developer Mode](https://discuss.frappe.io/t/set-developer-mode/3922)

**Impact:**
- Affects: New developers, mode-switchers
- Severity: Confusing, wastes time
- Context: Setup, mode transitions

**Workarounds:**
- Detailed documentation reading
- Community help

---

### Problem 6: No Sync Between Local and Cloud

**Score: 78/100** (P2 - Significant)

| Factor | Score | Notes |
|--------|-------|-------|
| Frequency | 8 | Recurring request |
| Severity | 8 | Fundamental gap |
| Addressable | 9 | Exactly what weg clone does |
| Alignment | 10 | Core feature |

**Evidence:**

> "A user asked about software that can synchronize two servers at different locations to maintain the same database status" - [How Synchronize Local and Cloud instances](https://discuss.frappe.io/t/how-synchronize-local-and-cloud-instances/18302)

> "Custom permissions sync on new production sites but don't sync on migrate on the local development server" - [Sync permissions](https://discuss.frappe.io/t/sync-permissions/47446)

**Impact:**
- Affects: Anyone with local + cloud environments
- Severity: High - manual process, error-prone
- Context: Multi-environment workflows

**Workarounds:**
- Manual export/import
- Custom app deployment
- Accept environments drifting apart

---

## Patterns Observed

1. **The "create an app" answer is unsatisfying** - Every thread recommends it, but it's heavyweight for customizations
2. **Frappe Cloud is production-only** - No real development story
3. **Database-stored customizations are the root problem** - Hard to version control, sync, diff
4. **Team workflows are broken** - No good collaboration story for customizations
5. **Migration is scary** - Fear of losing customizations on upgrade

## Validation for 002-remote-clone

This mining strongly validates the spec. The problems are:
- Real (extensive evidence)
- Frequent (years of complaints)
- Severe (blocking or highly painful)
- Addressable (weg clone solves most)

**Recommended priority:** P0 - This is the #1 pain point for the target audience.

---

## Beads to File

| ID | Title | Category | Score | Priority |
|----|-------|----------|-------|----------|
| 1 | Cannot develop on Frappe Cloud | remote-dev | 85 | P1 |
| 2 | Version control of customizations broken | workflow | 82 | P1 |
| 3 | Client scripts not portable | customization | 75 | P2 |
| 4 | Custom app overhead for simple customizations | workflow | 70 | P2 |
| 5 | Developer mode confusion | cli-ux | 58 | P3 |
| 6 | No sync between local and cloud | remote-dev | 78 | P2 |

---

## Sources

- [Using Frappe Cloud for development](https://discuss.frappe.io/t/using-frappe-cloud-for-development/95397)
- [How to manage erpnext customizations [version control]](https://discuss.frappe.io/t/how-to-manage-erpnext-customizations-version-control/25008)
- [Best way to manage client scripts on core doctypes](https://discuss.frappe.io/t/best-way-to-manage-client-scripts-on-core-doctypes/99967)
- [What is the best practice for custom scripts?](https://discuss.frappe.io/t/what-is-the-best-practice-for-custom-scripts/11983)
- [How to enable developer mode in frappe cloud](https://discuss.frappe.io/t/how-to-enable-developer-mode-in-frappe-cloud/124602)
- [How Synchronize Local and Cloud instances](https://discuss.frappe.io/t/how-synchronize-local-and-cloud-instances/18302)
- [Sync permissions](https://discuss.frappe.io/t/sync-permissions/47446)
- [Developer mode to Production mode in erpnext 13](https://discuss.frappe.io/t/developer-mode-to-production-mode-in-erpnext-13/79412)
