---
title: Local live dogfood evidence must retain counts
date: 2026-08-20
category: logic-errors
module: devonthink publishing workflow
problem_type: logic_error
component: development_workflow
symptoms:
  - "Phase 5 was marked skipped without a local execution count"
root_cause: missing_workflow_step
resolution_type: workflow_improvement
severity: medium
tags: [devonthink, dogfood, publishing, validation]
---

# Local live dogfood evidence must retain counts

## Problem

PR #1407 had `dogfood-results.json`, but its Phase 5 marker only recorded a
network-unreachable skip. That did not give a reviewer enough evidence that the
local-only DEVONthink integration had actually run.

## Symptoms

- A maintainer requested either spaced local live evidence with counts or a
  clear local-only substitute.
- The old acceptance file reported `status: skip` rather than an executed test
  matrix.

## What Didn't Work

- Keeping only the generation-host network explanation did not establish the
  result of the available local runtime.
- Committing raw dogfood output would have exposed temporary local paths and
  sampled command output.

## Solution

Run the full local gate and commit two bounded proof artifacts:

```bash
cli-printing-press dogfood --dir "$PWD" --live --level full --timeout 120s \
  --write-acceptance .manuscripts/<run>/proofs/phase5-acceptance.json --json
```

Keep the acceptance marker and create a sanitized publish gate containing only
the verdict, counts, and per-command status. In PR #1407 the final local gate
reported 122 passed, 107 safely skipped, and 0 failed.

Input validation added in
`library/productivity/devonthink/internal/cli/helpers.go` rejects malformed
UUIDs before read commands build a request, while
`library/productivity/devonthink/internal/cli/tail.go` rejects unknown resource
names before it creates the client. Missing arguments remain command-specific:
the UUID validator intentionally passes an empty argument slice through.

## Why This Works

The proof distinguishes a local-only dependency from an untested feature:
reviewers can inspect a reproducible matrix result without receiving local
filesystem details or document content. The paired regression tests preserve
the existing help and JSON usage contracts for missing arguments.

## Prevention

- When an integration is local-only, rerun its local live gate before replying
  to a review and include pass, skip, and fail counts.
- Sanitize command output before committing proof artifacts.
- Test invalid and missing positional arguments separately; the latter may have
  a command-specific response contract.

## Related Issues

- PR #1407
