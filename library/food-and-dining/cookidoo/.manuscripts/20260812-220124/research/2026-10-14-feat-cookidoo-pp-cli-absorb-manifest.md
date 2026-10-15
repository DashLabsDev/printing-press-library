# Cookidoo CLI Absorb and Transcendence Manifest

## Absorbed public capabilities
| Capability | CLI path | Scope |
|---|---|---|
| Public recipe details | `recipes details` | Structured read of one public recipe |
| Ingredient view | `recipes ingredients` | Normalized ingredient data |
| Nutrition view | `recipes nutrition` | Structured nutrition data |
| Multi-recipe ingredient plan | `recipes plan` | Local aggregation across selected public recipes |
| Shopper candidate handoff | `recipes shopper` | Read-only catalog search via an explicitly selected Shopper CLI; results require review |

## Transcendence feature
| Feature | Command | Score | Evidence |
|---|---|---:|---|
| Multi-Recipe Ingredient Plan | `recipes plan` | 6/10 | `dogfood-output.json` records live happy-path and JSON-fidelity passes for `recipes plan r68517 r68522 --json`; unit coverage is in `internal/cli/cookidoo_views_test.go`. Public endpoint responses are single-recipe, so the combined view is produced locally. |

## Explicitly excluded
Authenticated Cookidoo account data, personal recipe collections, meal-plan changes, and purchases are excluded. Public search is deferred because there is no stable credential-free contract.
