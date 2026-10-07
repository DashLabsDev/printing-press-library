# costco-sameday research notes

## Cancel
- **REST** `PUT /api/v2/orders/{orderId}/cancel?source=web` (200 OK in cancel sniff)
- Body: `{"cancellation_reason":{"costRelated":""}}`
- Not a GraphQL mutation. GraphQL `CustomerCancelSelections` is options-only.

## Place
- GraphQL `FinalizeCheckout` dual-gated: `--yes` + `--confirm-charge`; `--dry-run` never POSTs.

## Never in git
- HARs, cookies, credentials.toml, card data
