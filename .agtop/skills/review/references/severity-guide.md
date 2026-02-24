# Issue Severity Classification Guide

Use this guide when classifying review issues. Think carefully about the real impact on the user and the feature before assigning a severity level.

## Severity Levels

### blocker

The issue **prevents the work from being released**. Use this classification only when:

- The feature does not function as specified in a way that is visible to users
- The implementation is fundamentally wrong (e.g., wrong data source, incorrect business logic)
- The user experience is broken or significantly degraded (e.g., page crashes, data loss, inaccessible features)
- Security vulnerabilities are introduced (e.g., exposed credentials, missing auth checks, injection vectors)
- Data integrity is compromised (e.g., incorrect calculations, corrupted state, lost records)

**Test:** If you shipped this today, would users report it as a bug or would it cause real harm? If yes, it's a blocker.

### tech_debt

The issue **does not prevent release** but creates technical debt that should be addressed in a future iteration. Use this classification when:

- The implementation works but uses a pattern that will cause problems at scale
- Code duplication that should be abstracted but doesn't affect functionality
- Missing error handling for edge cases that are unlikely but possible
- Performance concerns that don't affect current usage but will matter later
- Incomplete type coverage or missing validation for non-critical paths
- Hardcoded values that should be configurable but work correctly as-is

**Test:** Does this work correctly today but will cause pain in 3-6 months? If yes, it's tech debt.

### skippable

The issue is **minor and non-blocking**. It's a real problem but not critical to the feature's core value. Use this classification when:

- Minor UI inconsistencies (spacing, alignment, color shade slightly off)
- Copy or wording that could be improved but is functional
- Missing nice-to-have features that weren't core requirements
- Code style preferences that don't affect functionality
- Documentation gaps for non-critical paths

**Test:** Would you mention this in a code review comment but still approve the PR? If yes, it's skippable.

## Decision Framework

When unsure about severity, work through these questions in order:

1. **Does the feature work as the spec describes?** No -> likely `blocker`
2. **Will this cause problems for users right now?** Yes -> likely `blocker`
3. **Will this cause problems for developers later?** Yes -> likely `tech_debt`
4. **Is this a preference or polish issue?** Yes -> likely `skippable`

## Common Mistakes

- **Over-classifying as blocker:** Not every deviation from the spec is a blocker. If the core functionality works and the deviation is cosmetic or minor, it's not a blocker.
- **Under-classifying blockers:** Security issues and data integrity problems are always blockers, even if they seem edge-case.
- **Mixing severity with effort:** A large refactor that's tech_debt is still tech_debt, not a blocker. Severity is about impact, not effort to fix.
