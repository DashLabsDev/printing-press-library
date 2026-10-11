# Home Depot CLI research brief

## API identity and product thesis

- Domain: Home Depot's signed-in consumer shopping website; the underlying endpoints are private and undocumented.
- Users: repeat Home Depot shoppers who want to find a specific product, manage a cart, and retrieve past purchases without navigating several web pages.
- Data profile: product identifiers and store-specific offers, an account cart, and order summaries with line items. Session cookies and order details are sensitive.
- Name: `homedepot`.
- Thesis: a scriptable shopper CLI that shows concrete products and cart state and retrieves purchase history using the shopper's own session. It should report when the website blocks or changes a private endpoint instead of claiming a successful mutation.

## Top workflows and table stakes

1. Search products by ordinary words, then see item ID, price, availability, and store context.
2. Inspect the current cart, add an item, change quantity, and remove an item with an explicit resulting cart state.
3. List recent online and in-store orders and inspect line items to find a past purchase.
4. Import an existing browser session and see whether it is present or expired without exposing cookie values.

Comparable tools set expectations for browser-session import, usable search results, real cart mutations, and order lookup. Instacart and Costco Same-Day offer search/cart flows; Walmart and Amazon Orders provide useful order-history and session-import patterns. The user requested search/cart and orders, with checkout and payment out of scope.

## Data layer

- Highest-gravity entities: product, offer/fulfillment, cart/item, order/line item, and session.
- Local store: a private 0600 session file and, if useful after live verification, a cache of the user's order summaries and line items for local search. Keep full addresses and payment details out of default output and cache.
- Sync cursor: order date range plus page token/offset, subject to the site's roughly 24-month history window; revalidate this against live behavior.
- Search: remote product query first; optional local order-line search once order retrieval works.

## Reachability risk

- High. The site relies on signed-in browser context and bot protection. An anonymous `curl` GraphQL search probe returned HTTP 403 on 2026-10-10 while Safari loaded the search page. A copied HTTP request from a browser session has not been proven to work in a CLI.
- Probe-safe read operation: `POST /federation-gateway/graphql?opname=searchModel` with a search-only query. Do not call cart mutations until their request contract and authorization are verified.
- Home Depot has no official consumer shopper OpenAPI discovered here. Community wrappers are the best available contract clues and may break when the site changes.

## User scope

Build a logged-in Home Depot shopper CLI for product search, cart view and changes, and purchase history. Do not implement checkout or payment in the first version. No publish request yet.

## Comparable printed CLIs

- Instacart: direct browser-session GraphQL replay for search and cart changes; login imports Chrome cookies. https://github.com/mvanhorn/printing-press-library/tree/main/library/commerce/instacart
- Walmart: purchase history, search, product, cart view; cart changes are link-only. Its CLI explicitly refuses mutation requests. https://github.com/mvanhorn/printing-press-library/tree/main/library/commerce/walmart
- Amazon Orders: cookie import including Safari for order-history reads. https://github.com/mvanhorn/printing-press-library/tree/main/library/commerce/amazon-orders
- Costco Same-Day: authenticated GraphQL cart and orders. https://github.com/mvanhorn/printing-press-library/tree/main/library/commerce/costco-sameday

## Home Depot sources

- Home Depot has no discovered official consumer shopper API or OpenAPI spec. The Pro integrations page serves a different audience. https://www.homedepot.com/c/pro-integrations
- MIT Go library for order history: https://github.com/fnziman/homedepot-go . Uses logged-in cookie replay, POST orderhistory and orderdetails, supports online and in-store details, and has a roughly 24-month history limit. Its documented cookie export is incomplete for HTTP-only cookies; prefer a browser cookie store import or explicit cookie-file import with local permissions.
- MIT order exporter and API mapping: https://github.com/joshellissh/homedepot-history
- Home Depot MCP browser connector (no license detected): https://github.com/markswendsen-code/mcp-homedepot . Conceptual reference only; do not copy code.
- MIT OpenWeb Home Depot search adapter: https://github.com/imoonkey/openweb/blob/main/src/sites/homedepot/openapi.yaml . This spec's JSON response describes its own page adapter, so it must not be passed to Printing Press as if Home Depot served that JSON.
- Public cart JS bundle observed at https://assets.thdstatic.com/experiences/cart/mutation-helper.b105d9370ebaa0c3c603.js . It names addToCart, updateCart, deleteItem mutations and a CartInfoRequest; its bundle is an implementation clue, not a verified request/response contract.

## Direct observations on 2026-10-10

- Signed-in Safari session loaded Home Depot purchase history and product search pages. No session credentials are stored in this run directory.
- Product search page called POST https://www.homedepot.com/federation-gateway/graphql?opname=searchModel . Its request used keyword, paging and store-context variables and returned products with item identifiers, prices, and availability.
- Cart page called POST https://apionline.homedepot.com/federation-gateway/graphql?opname=getCart . Other observed operations were uds, endCap and getAllSaveForLaterItems.
- A product search add-button click did not yield a captured addToCart request or a visible pencil in the cart. Do not treat it as a successful cart mutation.
- Plain curl of Home Depot search HTML returned an error page despite Safari loading search; direct HTTP CLI transport needs a fresh authenticated reachability proof.
- A later cart page showed one line item at quantity 1. Incrementing it to 2 produced a successful `POST https://apionline.homedepot.com/federation-gateway/graphql?opname=updateCart` response (HTTP 200). The request contained `variables.cartRequest.localization.primaryStoreId`, `items.pickup` with item ID, line-item ID, type, quantity, location, and store, plus `requestContext`. Decrementing restored quantity 1, and the page returned to `CART (1)`. This is an observed update contract; it is not yet a proven CLI replay.

## Authorized read-only CLI check on 2026-10-10

- The user approved importing the existing Safari session into a private local file and replaying read-only search, cart, and order requests. `auth import` saved 48 Home Depot cookies under `~/Library/Application Support/homedepot-pp-cli/cookies.json` with mode 0600. `auth status` showed `session_present: true` and `live_validated: false`.
- `orders list` reached Home Depot but returned HTTP 400 `OMS_GLOBAL_ERROR_00002` (invalid customer account number). The third-party order client's mapping from `THD_CUSTOMER` is not validated for this session.
- The prototype product search request returned HTTP 206. The full generated CLI with imported cookies and Chrome-style transport reached the GraphQL gateway, but its guessed search query returned `Generic Errors API`; it did not produce products.
- The signed-in Safari cart page used a GraphQL `getCart` operation with `requestContext` and `cartInfo` containing `itemCount`. A minimal read-only `getCart` query from the full CLI to `apionline.homedepot.com` also returned `Generic Errors API`, rather than cart data. The browser's longer query and context may be required; direct replay remains unproven.
- No cart changes or order placement occurred during this check. None of search, cart read, or order history passed live CLI validation.

## User-provided Safari HAR follow-up on 2026-10-10

- The HAR contained successful browser `addToCart` and `getCart` GraphQL requests and successful OMS `orderhistory` and `orderdetails` requests. The raw HAR stays in the user's Downloads folder; no credentials or personal response bodies were copied into this run.
- `addToCart` used `cartRequest` and `requestContext`; `getCart` used `requestContext` with a roughly 9.7 KB GraphQL query. These are observed browser contracts, not verified direct CLI replays.
- The OMS `orderhistory` request included `customerAccountId` from the signed-in customer cookie, plus paging, sort, filter, date range, and timezone fields. The earlier third-party client omitted the account field, explaining its HTTP 400 response. Home Depot also serializes store, register, and transaction IDs as numbers and labels in-store origins with a store name, which that client did not parse correctly.
- A direct read-only replay of the observed OMS request returned HTTP 200 and `orderCount: 20`. The CLI's new `orders list` returned seven orders for 2026-09-01 through 2026-10-10. `orders get` returned one online order with one line item and one in-store order with four line items; the CLI output intentionally omits addresses and payment details.
- Exact `getCart` replay with the HAR query and relevant headers still returned HTTP 206 with `Generic Errors API`, including under the generated Chrome-style transport. Direct GraphQL product search has the same unresolved transport or clearance behavior.
- The HAR's server-rendered search page embeds ordered product references in `window.__APOLLO_STATE__`. The signed-in Chrome-style transport fetched the search HTML with HTTP 200. The new `products search` command parsed 24 products with prices from a live `carpenter pencils` query. This validates product search through HTML, even though direct GraphQL search remains blocked.
- A fresh Safari cookie import plus the HAR's `getCart` query, selected browser headers, and all imported cookies still returned HTTP 206 with `Generic Errors API`. This first replay omitted the browser's `sec-ch-ua` hint, causing a Chrome identity mismatch.

## Cart replay diagnosis and fix on 2026-10-10

- The browser HAR returned HTTP 200 with `data.cartInfo`. Replaying the same read-only query with the full browser headers and imported cookies also returned HTTP 200. Omitting `sec-ch-ua`, or sending Chrome 145 client hints beside a Chrome 154 user agent, returned HTTP 206 with `Generic Errors API`. Supplying a consistent Chrome 145 user agent, client hints, and platform restored HTTP 200.
- Omitting `X-Experience-Name: my-cart` returned HTTP 403. The working request also carries `Origin`, `Referer`, and `Sec-Fetch-*` context. A reduced read-only query for item count, item identifiers, names, quantities, prices, and total succeeded with those headers and the imported cookies.
- The new `cart show` command uses the generated Chrome-compatible transport and this request context. It passed a live check with two line items and a total, while excluding customer, address, and payment data from the query and output. This validates cart reads only; cart mutations still need separate implementation and proof.

## Initial command surface

- `auth import` / `auth status`: import the user's own signed-in session to local storage and check expiry without printing cookie values.
- `products search <query>` / `products get <item-id>`: store-aware item IDs, prices, stock and fulfillment.
- `cart show`, `cart add <item-id> --quantity N`, `cart update`, `cart remove`: use only captured and verified request contracts. `cart add` must report a cart ID or changed item count; no silent success. No checkout/payment commands.
- `orders list --from --to`, `orders get <order-id>`: order summaries and line items, excluding addresses and payment fields by default.

## Open blockers to a shippable CLI

1. Capture authenticated deleteItem request. `addToCart` and `updateCart` are browser-observed, but direct CLI mutation replay is unproven.
2. Implement and verify cart add/update/remove from browser-observed contracts, with read-after-write confirmation and no checkout path.
3. Run full Printing Press dogfood and review gates before promotion. Product search, cart show, and orders list/get pass focused live read checks. No automated tests were added or run.
