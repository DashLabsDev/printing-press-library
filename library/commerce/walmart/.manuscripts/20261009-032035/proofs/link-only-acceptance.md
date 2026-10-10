# Walmart CLI — link-only cart acceptance

Run id: 20261009-032035

## Design

Cart mutations are not sent over orchestra HTTP. The only cart-change surface is
`cart link` (alias `cart add`), which builds one Walmart affiliate add-to-cart URL:

`https://affil.walmart.com/cart/addToCart?items=ID|qty,ID|qty`

- Prints the URL by default
- `--open` / `--launch` is opt-in; refused under Printing Press verify/dogfood harnesses
- Labelled `mcp:read-only`
- Documents that the link ADDS / increments quantity on whatever is already in the browser cart
- No CDP / browser driver; no `updateItems` / `reserveSlotMutation` / checkout

## Live proof (maintainer)

Multi-item link confirmed by the maintainer in a signed-in browser, 3/3 items added.

Public product ids used for that check (not account-specific): bananas, pears, and paper towels from the public catalog. No account identifiers, addresses, phones, or order ids are recorded here.

The same multi-item link also works signed out: items land in a guest cart and carry over into the account cart on sign-in (maintainer-confirmed).

## Reads kept

orders, search, product, store list, cart view, slots list — orchestra GETs with imported browser cookies.
