# Walmart research notes (run 20261009-032035)

Source: one partial read-only HAR capture of walmart.com (kept off-repo, not published).

## Captured orchestra operations (read)
- cph: PurchaseHistoryV3, ItemHistory, accountLandingPage
- orders: getOrder
- Other read ops seen but not used: FetchNotifications, multiReviews, getContentLayoutModules, getAmendableOrder, GlobalIntentCenter, shippingCountryList, WPlusSplashPage, getHeartedItems, ad display ops

## Mutations seen
- MergeAndGetCart (cartxo) - fired automatically by the web app on page load; denylisted, never called by the CLI.

## Search
No orchestra search op; /search is server-rendered and parsed from __NEXT_DATA__.

## Second capture (read-only)
- pdp: ItemById (GET, used by product get); ItemByIdBtf (POST, below-the-fold content; blocked by the guard as a non-GET, not needed)
- home: nearByNodes (store list), getCart (cart view), PostCartLoadPage, GetUserResidency, wPlusOptinStatus, getAmendableOrder
- cartxo: getSFL, GetUserResidency, accountLandingPage
- snb: Search (client-side page 2 and brand filter); the CLI keeps the server-rendered page and passes facet/sort as URL parameters
- Ad POSTs to /orchestra/home/graphql and /swag/graphql (ignored)
- No mutation operations in this capture.

Search filters use facet ids of the form type:value (e.g. brand:Great Value); the list comes from the SearchSortFilterModule allSortAndFilterFacets config.

## Still missing
- Slot drawer: "Reserve a time" never appeared (cart empty; captured product not available for pickup or delivery). slots stays a stub.
- Non-empty cart line items; search with multiple filters; getOrderLedger.

## Live verification (light, read-only, spaced calls)
- Run 1: doctor, orders list (one page), orders get (one order), search: all succeeded with no bot challenge.
- Run 2: doctor succeeded; product get returned HTTP 412 on its first call and testing stopped (store list, cart view and filtered search not run).

## Request-fidelity review (offline, after the 412)
Name-only comparison of CLI requests (recorded with a no-network transport) against the browser's captured requests:
- ItemById path was missing the trailing /ip/<itemId> segment the web app uses (fixed).
- CLI user agent was Windows Chrome 148; the session cookies were minted by Linux Chrome 154. UA, sec-ch-ua, client hints, accept-language, cache-control, pragma and priority now match.
- x-o-platform-version was one web build behind (fixed; overridable).
- Cookie jar sent 21 names twice and two stale anti-bot cookies from an earlier session; login now replaces the jar with the browser's exact rows and the jar is the single cookie source.
- Not matched by design: baggage and device_profile_ref_id (session/device identifiers), HTTP/2, Chrome's TLS fingerprint and accept-encoding list.

## Cart and slot capture (third capture, Pickup mode)

- Operations seen: `cartxo/updateItems` and `orders/updateItems` (same persisted hash; add, quantity change, removal = quantity 0; quantity is absolute), `cartxo/getSlots` (GET, headers `x-o-default-slot-intent` and `x-o-in-home-slots`), `cartxo/reserveSlotMutation` (POST), `cartxo/MergeAndGetCart` (POST, still blocked), `cartxo/GetBookSlotPage` (content layout only).
- Mutation POST bodies carry only `variables` (persisted query via path hash) plus `Origin`.
- `reserveSlotMutation` sends the cart id, the slot's `slotMetadata`, and a flattened `selectedSlot` (slot fields + price fields flattened to display/value pairs, `available` -> `isAvailable`, `slotDay`, `timeWindow`). Rebuilt offline from the getSlots slot, it matches the captured variables exactly.
- No release/unreserve request was sent and the UI offered none; the reservation carries an `expiryTime` hold. The web bundle defines a `cancelReservation` persisted mutation used by the drone-delivery confirmation flow; it was never observed on the wire, is not implemented, and is denylisted along with `createReservation`.
- The cart id appears only in `getCart`/`getSlots` request variables and in mutation responses (`MergeAndGetCart`, `updateItems`, `reserveSlot` -> `id`). No read-only response returns it, so the CLI remembers it from cart responses after the user supplies it once.
- Live read-only checks (2026-10-09 CT, ~90 s apart, no challenge): cart view 10:36, slots list 10:38, product get 10:39 (the earlier 412 did not recur after the request-fidelity fix). No live writes were run.

## Live cart-write test attempt (2026-10-09 11:08 CT)

- Before the test, the box browser no longer held the session's `auth` cookie (other long-lived cookies remained), so a fresh cookie import was refused by the CLI's auth-cookie check.
- The first step (read-only `cart view`, using the previously imported session) failed with the connection closed by Walmart (EOF), no HTTP status. Treated as a possible bot block: testing stopped, no writes were sent.
- Fix: transport failures on Walmart hosts now report the operation path without the URL query and are classified as a possible bot block.

## Session cookie and live cart-write attempt (2026-10-09 11:25-11:35 CT)

- The signed-in session is the `auth` cookie on www.walmart.com: httpOnly, about 30-minute expiry, renewed by site activity and minted from the long-lived identity.walmart.com session (`_auth`, `hasAuth`, `ACID` there; not sent to www). CID/SPID/customer on www.walmart.com mark a signed-in device. An idle browser simply has no `auth` row; it was not renamed.
- Reads (getCart, ItemById) succeeded with the imported session. The first `updateItems` POST was closed by Walmart without a response (EOF), matching the earlier drop. The request header names match the browser's except the deliberately unforged `baggage` and `device_profile_ref_id`; the remaining differences are transport-level (HTTP/1.1 vs the browser's HTTP/2, Go's TLS handshake) and browser-sensor state. The add was not committed; the cart was verified empty afterwards. Write commands remain unverified live.

## Hybrid browser writes (2026-10-09)

- Affiliate add-to-cart: `GET https://affil.walmart.com/cart/addToCart?items=<id>|<qty>` 307s to www, then `/affil/addToCart` POST from the page with `{items:[{itemId,quantity}]}`. Multi-item format in the affil JS: comma-separated, each `id|qty` or `id_qty`. Link increments. `www.walmart.com/cart/addToCart` 404s.
- Direct orchestra `updateItems` POST from the CLI is dropped (EOF); not used for writes anymore.
- Browser driver (chromedp remote) clicks allowlisted cart/bookslot controls; checkout URLs denied. Selectors from cart/bookslot bundles are fragile.

## Link-only cart (v1)

Writes are link-only: `cart link` prints an affil.walmart.com multi-item add-to-cart URL; the CLI never mutates the cart via API. CDP/browser driving and consented orchestra cart mutations were removed. Maintainer confirmed a 3-item link added 3/3 items in a signed-in browser. See proofs/link-only-acceptance.md.
