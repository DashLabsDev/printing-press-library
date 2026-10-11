Polish pass (printing-press-polish, forked):
  Scorecard: 96 → 96; Verify: 100% → 100% (214/214); Dogfood: FAIL → PASS; tools-audit 1 → 0; gosec (hand-written) 1 → 0
  Fixes: manifest description synced to research.json (.printing-press.json, manifest.json, agentcookie.toml); usage top Short expanded; narrow #nosec G304 on user-chosen context file read.
  Skipped: 46 gosec in generated files (retro); live-check wait 401 placeholder id; preview up/prune destructive; mcp_description_quality/mcp_token_efficiency 0, type_fidelity/dead_code 5 (structural).
  ship_recommendation: ship
