# Cookidoo CLI Research Brief

## API identity and public scope
Cookidoo is a recipe service. This package exposes public recipe-page data only: recipe details, ingredient and nutrition views, and a local multi-recipe ingredient plan. It does not access account data or mutate Cookidoo state.

## Evidence
- Official Cookidoo Explore page: https://cookidoo.thermomix.com/foundation/en-US/explore
- Official public recipe page used for read-only validation: https://cookidoo.thermomix.com/recipes/recipe/en-US/r68517
- Public recipe pages provide structured recipe data without a logged-in account.
- The stable public contract supports retrieving a single recipe by ID or public recipe URL. Public search remains deferred because its browser-rendered flow does not expose a stable credential-free contract.
- The package README and current CLI command tree document `recipes details`, `recipes ingredients`, `recipes nutrition`, `recipes plan`, and a Shopper catalog handoff.

## Primary workflows
1. Fetch one public recipe as structured JSON.
2. Inspect its ingredients or nutrition.
3. Combine ingredient lists for multiple selected recipes into one plan.
4. Optionally pass ingredients to a separate Shopper catalog lookup for candidate review.

## Transcendence
`recipes plan` merges normalized ingredient data from multiple recipe fetches into one structured plan. The public endpoint returns one recipe per request and has no multi-recipe aggregation response. Dogfood's full live matrix exercised `recipes plan r68517 r68522 --json` and its JSON fidelity check successfully.

## Boundaries and risks
- No Cookidoo credentials are used for public recipe reads.
- Authenticated account resources, personal meal plans, writes, and purchases are excluded.
- Catalog search is a separate read-only Shopper handoff; candidate matches require human review.
- Public recipe availability and locale behavior can vary by region.

## Build priorities
The package is complete for its public read-only scope: structured recipe retrieval, ingredient and nutrition views, and a multi-recipe ingredient planner. The private account and meal-planning features mentioned in historical notes are intentionally not represented in public metadata.
