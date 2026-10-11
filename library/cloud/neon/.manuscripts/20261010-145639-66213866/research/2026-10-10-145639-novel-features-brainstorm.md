# Novel features brainstorm (subagent output)
## Customer model
- Priya (branch-per-PR full-stack dev): branches pile up; no Last Active; cleanup id-by-id.
- Marcus (platform engineer, multi-org fleet): no tree view, no offline search, N×M calls for inventory; can't spot computes that never suspend.
- Dana (bill owner): no consumption command in neonctl; can't attribute compute-hour spikes by project.
- Agent (ephemeral DB provisioner): polls operations by hand; no distinct exit codes.
## Candidates (pre-cut)
1 tree, 2 stale, 3 prune, 4 usage trend, 5 usage top, 6 wait, 7 preview up, 8 compute audit, 9 restore-window, 10 snapshot coverage, 11 changes, 12 lineage, 13 access matrix, 14 usage estimate.
## Survivors and kills
### Survivors
stale 9, tree 8, usage trend 8, wait 8, usage top 7, preview up 7, prune 7, compute audit 6 — all hand-code.
### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| restore-window | Thin wrapper over project get history_retention_seconds | preview up |
| snapshot coverage | No demand evidence; 4/10 | compute audit |
| changes | Speculative; tree shows created_at | tree |
| lineage | Subset of tree | tree |
| access matrix | No evidence, tangential | tree |
| usage estimate | Needs hardcoded price table not in API; unverifiable | usage top |
