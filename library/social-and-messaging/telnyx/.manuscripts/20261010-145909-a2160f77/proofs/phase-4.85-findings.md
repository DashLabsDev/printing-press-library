# Phase 4.85 output review
status: WARN (1 warning) — fixed
- silent-empty-local-store: spend/failures now emit a `note` when zero records are in the window; conversation echoes counterpart + note when no mirror; all local commands print a stderr sync hint (openMirror / hintIfUnsynced). numbers audit and inbox unanswered return [] with the stderr hint (list shape kept for agents).
