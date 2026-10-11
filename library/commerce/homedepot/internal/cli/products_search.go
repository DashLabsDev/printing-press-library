// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0.
// Search products through the server-rendered Home Depot result page.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/client"
	"github.com/spf13/cobra"
)

type homeDepotProduct struct {
	ItemID       string   `json:"item_id"`
	Name         string   `json:"name"`
	Brand        string   `json:"brand,omitempty"`
	Model        string   `json:"model,omitempty"`
	URL          string   `json:"url"`
	Price        *float64 `json:"price,omitempty"`
	Availability string   `json:"availability"`
}

// pp:data-source live
func newNovelProductsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use: "search <query>", Short: "Search by query in the signed-in store context; return first-page IDs, names and prices (limit 1-24)",
		Example:     "  homedepot-pp-cli products search pencils --limit 5 --json",
		Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "true", "pp:happy-args": "query=pencils;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usageErr(fmt.Errorf("provide one product search query"))
			}
			query := strings.TrimSpace(args[0])
			if limit < 1 || limit > 24 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 24 (one search page)"))
			}
			if query == "" {
				return usageErr(fmt.Errorf("search query cannot be empty"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "products search")
			}
			if _, _, err := loadHomeDepotSession(); err != nil {
				return err
			}
			c, err := newHomeDepotSessionClient(flags)
			if err != nil {
				return err
			}
			c.NoCache = true
			raw, err := c.GetWithHeaders(cmd.Context(), "/s/"+url.PathEscape(query), nil, map[string]string{client.HTMLResponseHeader: "true"})
			if err != nil {
				return homeDepotRequestError("read Home Depot search page", 0, err)
			}
			products, err := parseHomeDepotSearchPage(raw)
			if err != nil {
				return err
			}
			if len(products) > limit {
				products = products[:limit]
			}
			return flags.printJSON(cmd, products)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 24, "Maximum products from the first search page (1-24)")
	return cmd
}

func parseHomeDepotSearchPage(page []byte) ([]homeDepotProduct, error) {
	marker := []byte("window.__APOLLO_STATE__")
	pos := bytes.Index(page, marker)
	if pos < 0 {
		return nil, fmt.Errorf("Home Depot search page has no product state")
	}
	page = page[pos+len(marker):]
	eq := bytes.IndexByte(page, '=')
	if eq < 0 {
		return nil, fmt.Errorf("Home Depot product state is incomplete")
	}
	var state map[string]json.RawMessage
	if err := json.NewDecoder(bytes.NewReader(page[eq+1:])).Decode(&state); err != nil {
		return nil, fmt.Errorf("decode Home Depot product state: %w", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(state["ROOT_QUERY"], &root); err != nil {
		return nil, fmt.Errorf("decode Home Depot search root: %w", err)
	}
	searchRef := ""
	for k, v := range root {
		if strings.HasPrefix(k, "searchModel(") {
			var ref struct {
				Ref string `json:"__ref"`
			}
			if json.Unmarshal(v, &ref) == nil {
				searchRef = ref.Ref
				break
			}
		}
	}
	if searchRef == "" {
		return nil, fmt.Errorf("Home Depot search results were not present")
	}
	var search map[string]json.RawMessage
	if err := json.Unmarshal(state[searchRef], &search); err != nil {
		return nil, fmt.Errorf("decode Home Depot search results: %w", err)
	}
	var refs []struct {
		Ref string `json:"__ref"`
	}
	for k, v := range search {
		if strings.HasPrefix(k, "products(") {
			if err := json.Unmarshal(v, &refs); err != nil {
				return nil, fmt.Errorf("decode Home Depot product references: %w", err)
			}
			break
		}
	}
	if refs == nil {
		return nil, fmt.Errorf("Home Depot search page has no product list")
	}
	products := make([]homeDepotProduct, 0, len(refs))
	for _, ref := range refs {
		var item map[string]json.RawMessage
		if json.Unmarshal(state[ref.Ref], &item) != nil {
			return nil, fmt.Errorf("a Home Depot product record could not be parsed")
		}
		var identifiers struct {
			ItemID string `json:"itemId"`
			Name   string `json:"productLabel"`
			Brand  string `json:"brandName"`
			Model  string `json:"modelNumber"`
			URL    string `json:"canonicalUrl"`
		}
		if json.Unmarshal(item["identifiers"], &identifiers) != nil || identifiers.ItemID == "" || identifiers.Name == "" {
			return nil, fmt.Errorf("a Home Depot product has incomplete identifiers")
		}
		product := homeDepotProduct{ItemID: identifiers.ItemID, Name: identifiers.Name, Brand: identifiers.Brand, Model: identifiers.Model, Availability: "unknown", URL: "https://www.homedepot.com" + identifiers.URL}
		for k, v := range item {
			if strings.HasPrefix(k, "pricing(") {
				var pricing struct {
					Value *float64 `json:"value"`
				}
				if json.Unmarshal(v, &pricing) == nil {
					product.Price = pricing.Value
				}
				break
			}
		}
		products = append(products, product)
	}
	if len(refs) > 0 && len(products) == 0 {
		return nil, fmt.Errorf("Home Depot search products could not be parsed")
	}
	return products, nil
}
