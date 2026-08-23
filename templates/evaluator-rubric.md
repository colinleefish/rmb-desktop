# Evaluator Rubric — score after feature completion

> Score each completed feature (sprint) on 4 dimensions. **Every dimension must reach B or above.**
> A C or D is not a failure verdict — it's a signal: record it in `docs/quality-document.md` and,
> if the same dimension scores C/D twice in a row for a module, promote a fix feature.

| Dimension | A | B | C | D |
|---|---|---|---|---|
| **Correctness** | All layers green first run; no rework | Layers green after minor fixes | Needed repair-hint rescue; edge cases miss | Claimed done without passing layers |
| **Arch compliance** | No boundary violations; anticipated new rule | `check-arch` green, no new patterns | Violation caught by reviewer (not harness) | Rule violated knowingly |
| **Test coverage** | New behavior covered; failing-test-first where practical | New happy paths covered | Only touched existing tests | No tests touched |
| **Verification evidence** | `evidence` recorded + trace logged + docs same commit | Evidence recorded | Evidence missing from feature_list | Feature passed by hand-edit |

## Recording

Append to the feature's issue close comment: `score: <dim=A|B|C|D> ×4, note: <one line>`.
Update `docs/quality-document.md` for the touched module.
