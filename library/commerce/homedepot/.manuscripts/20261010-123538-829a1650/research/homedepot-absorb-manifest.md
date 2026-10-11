# Approved Home Depot shopper scope

The owner selected search/cart and order history, then asked to finish the draft. This retains that scope. Checkout, payment, delivery scheduling, inventory sweeps, monitoring, and new local purchase-history storage are excluded.

## Absorbed
| Feature | Best source | Our implementation | Added value |
|---|---|---|---|
| Browser session | homedepot-go cookie decoding; Printing Press cookie transport | homedepot-pp-cli auth import | Safari import with expiry and verified account read |
| Chrome session | Printing Press cookie auth | homedepot-pp-cli auth login | Standard Chrome extraction to the same cookie jar |
| Product search | observed SSR Apollo state; mcp-homedepot search workflow | homedepot-pp-cli products search | Ordered product IDs, prices, explicit unknown availability |
| Cart view | owner HAR getCart | homedepot-pp-cli cart show | Omits account, address and payment fields |
| Cart add | owner HAR addToCart | homedepot-pp-cli cart add | Explicit fulfillment and confirmed resulting state |
| Cart update | Home Depot static mutation bundle, live add/update/remove proof passed; exact original cart restored (see cart-lifecycle-proof.md) | homedepot-pp-cli cart update | Exact line selection and quantity verification |
| Cart remove | Home Depot static mutation bundle, live add/update/remove proof passed; exact original cart restored (see cart-lifecycle-proof.md) | homedepot-pp-cli cart remove | Exact line selection and absence verification |
| Order summaries | homedepot-go, owner HAR orderhistory | homedepot-pp-cli orders list | Bounded pagination with account ID from session |
| Order details | homedepot-go, owner HAR orderdetails | homedepot-pp-cli orders get | Online and in-store references; excludes address/payment |

## Scoped behavior improvements
| Feature | Command | Buildability | Evidence |
|---|---|---|---|
| Confirmed mutation results | cart add | hand-code | Mutation followed by read and quantity comparison |
| Fulfillment-aware targeting | cart update | hand-code | Fresh line ID and observed type/location fields |
| Honest availability | products search | hand-code | Source fields only; absent availability stays unknown |
| Complete bounded order listing | orders list | hand-code | OMS paging and repeated-page/error detection |
| Verified private session status | auth status | hand-code | Local presence and expiry plus --verify protected OMS read |

Novel-feature subagent completed customer/candidate/cut analysis. Additional reorder, cache, watch and project commands were cut as outside owner scope. These are improvements to approved commands, not an expansion.
