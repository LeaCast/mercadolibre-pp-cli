// Copyright 2026 leandrodc. Licensed under Apache-2.0. See LICENSE.
//
// MANUAL PATCH — see .printing-press-patches.json id="items-search-workflow".
//
// ML deprecated /sites/{site_id}/search in 2024 (HTTP 403 even with valid
// token). The only path to real listings with price is:
//   /products/search → /products/{id}/items → aggregate → filter → sort
// This file orchestrates that workflow client-side. Filters and sort are
// applied AFTER fan-out because /products/{id}/items doesn't accept them.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"mercadolibre-pp-cli/internal/client"
)

// noiseDomainFragments — substrings inside a catalog domain ID that
// represent secondary accessories users almost never want when their
// query doesn't explicitly mention them. Skipped during fan-out unless
// --domain-id is explicitly set.
//
// Kept narrow on purpose: only items that are clearly companion
// purchases (cases, covers, screen/camera protectors, films, glass
// guards). Things like headphones, cables, chargers, adapters,
// stands, mounts and holders are PRIMARY categories users routinely
// search for directly, so they stay in.
//
// Matched with strings.Contains (not HasSuffix), because ML composes
// domain IDs from multiple fragments — e.g. MLM-CELLPHONE_CASES_AND_COVERS
// contains "_CASES" but doesn't end in it.
var noiseDomainFragments = []string{
	"_CASES",
	"_COVERS",
	"_PROTECTOR",
	"_FILM",
	"_GLASS",
	"_SCREEN",       // replacement screens/displays
	"_DISPLAY",      // replacement displays
	"_BATTERY_CASE", // battery cases (companion accessory)
	"_LENS",         // lens accessories
	"_HOLDER",       // phone holders
	"_MOUNT",        // car mounts
	"_STAND",        // tablet/phone stands
}

const (
	maxSearchLimit  = 50
	maxCatalogLimit = 50
	itemsFanoutConc = 5
)

// supportedFilterKeys gates --filter parsing. Unknown keys fail loudly.
var supportedFilterKeys = map[string]bool{
	"price":         true,
	"condition":     true,
	"shipping_cost": true,
	"seller":        true,
	"currency":      true,
}

// supportedSorts gates --sort parsing. sold_desc removed: the
// /products/{id}/items endpoint that powers this workflow does not
// return reliable sold_quantity values (most rows arrive as 0), so
// sold-based ordering would be deceptive. Use ML's web interface
// directly if you need most-sold ranking.
var supportedSorts = map[string]bool{
	"price_asc":  true,
	"price_desc": true,
	"relevance":  true,
}

type searchFilter struct {
	priceMin     float64
	priceMax     float64
	hasPriceMin  bool
	hasPriceMax  bool
	condition    string // "new" | "used" | ""
	freeShipping bool
	sellerID     string
	currency     string // "ARS" | "USD" | ""
}

type listing struct {
	ItemID        string  `json:"id"`
	CatalogName   string  `json:"variant"`
	Price         float64 `json:"price"`
	Currency      string  `json:"currency"`
	Condition     string  `json:"condition"`
	FreeShipping  bool    `json:"free_shipping"`
	SellerID      int64   `json:"seller_id"`
	URL           string  `json:"url"`
	SoldQuantity  int64   `json:"sold_quantity"`
}

func newItemsSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagQ            string
		flagSiteID       string
		flagDomainID     string
		flagSort         string
		flagFilters      []string
		flagLimit        int
		flagCatalogLimit int
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Buscar publicaciones reales por keyword con filtros y orden (workflow catalog→items)",
		Long: `Buscar publicaciones reales en MercadoLibre por keyword.

Por que es un workflow y no un passthrough: ML deprecó el endpoint público
/sites/{site}/search en 2024 (responde 403). Este comando reconstruye la
funcionalidad: busca productos canónicos en el catálogo, luego trae las
publicaciones reales de cada producto, agrega, filtra y ordena del lado
del cliente.

Filtros soportados (--filter key=value, repetible):
  price=MIN-MAX        rango numerico (cualquiera de los dos puede ir vacio)
  condition=new|used   condicion de la publicacion
  shipping_cost=free   solo publicaciones con envio gratis
  seller=<id>          id numerico del vendedor
  currency=ARS|USD     moneda de la publicacion

Ordenes soportados (--sort):
  price_asc (default), price_desc, sold_desc, relevance`,
		Example: `  mercadolibre-pp-cli items search --q "motorola edge 60" --site-id MLA --sort price_asc --filter price=0-1000000 --limit 5
  mercadolibre-pp-cli items search --q iphone --site-id MLA --filter condition=new --filter shipping_cost=free`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagQ == "" {
				return usageErr(fmt.Errorf("--q es requerido"))
			}
			if flagSiteID == "" {
				return usageErr(fmt.Errorf("--site-id es requerido (ej MLA)"))
			}
			if !supportedSorts[flagSort] {
				return usageErr(fmt.Errorf("--sort %q no soportado; usa: price_asc, price_desc, sold_desc, relevance", flagSort))
			}
			if flagLimit < 1 || flagLimit > maxSearchLimit {
				return usageErr(fmt.Errorf("--limit fuera de rango (1-%d)", maxSearchLimit))
			}
			if flagCatalogLimit < 1 || flagCatalogLimit > maxCatalogLimit {
				return usageErr(fmt.Errorf("--catalog-limit fuera de rango (1-%d)", maxCatalogLimit))
			}

			filt, err := parseSearchFilters(flagFilters)
			if err != nil {
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			// Step 1: catalog search.
			catalogParams := map[string]string{
				"site_id": flagSiteID,
				"q":       flagQ,
				"limit":   strconv.Itoa(flagCatalogLimit),
			}
			if flagDomainID != "" {
				catalogParams["domain_id"] = flagDomainID
			}
			catalogRaw, err := c.Get(ctx, "/products/search", catalogParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			catalogProducts, err := extractCatalogProducts(catalogRaw)
			if err != nil {
				return fmt.Errorf("parsing catalog response: %w", err)
			}

			// Step 2: noise filter (skipped if user pinned a domain).
			if flagDomainID == "" {
				catalogProducts = filterNoiseProducts(catalogProducts)
			}

			// Step 3: fan out to /products/{id}/items with bounded concurrency.
			allListings := fanOutToItems(ctx, c, catalogProducts, flagSiteID)

			totalBeforeFilter := len(allListings)

			// Step 4: client-side filter.
			filtered := applyFilters(allListings, filt)

			// Step 5: sort.
			sortListings(filtered, flagSort)

			// Step 6: trim to --limit.
			if len(filtered) > flagLimit {
				filtered = filtered[:flagLimit]
			}

			// Step 7: output.
			return emitSearchResults(cmd, flags, filtered, flagQ, flagSiteID, flagSort, flagFilters, totalBeforeFilter)
		},
	}

	cmd.Flags().StringVar(&flagQ, "q", "", "Keyword de búsqueda (requerido)")
	cmd.Flags().StringVar(&flagSiteID, "site-id", "", "Codigo sitio (MLA, MLB, MLM, etc. — requerido)")
	cmd.Flags().StringVar(&flagDomainID, "domain-id", "", "Filtro de dominio del catalogo (ej MLA-CELLPHONES); desactiva el filtro de ruido")
	cmd.Flags().StringVar(&flagSort, "sort", "price_asc", "Orden: price_asc, price_desc, sold_desc, relevance")
	cmd.Flags().StringSliceVar(&flagFilters, "filter", nil, "Filtro key=value (repetible). Ver --help para keys soportadas")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximo de resultados finales (1-50)")
	cmd.Flags().IntVar(&flagCatalogLimit, "catalog-limit", 20, "Cuantos productos del catalogo fanout-ear (1-50)")

	return cmd
}

func parseSearchFilters(raw []string) (searchFilter, error) {
	var f searchFilter
	for _, kv := range raw {
		idx := strings.IndexByte(kv, '=')
		if idx <= 0 {
			return f, fmt.Errorf("filtro malformado %q (esperado key=value)", kv)
		}
		key := strings.TrimSpace(kv[:idx])
		val := strings.TrimSpace(kv[idx+1:])
		if !supportedFilterKeys[key] {
			return f, fmt.Errorf("filtro %q no soportado; usa: price, condition, shipping_cost, seller, currency", key)
		}
		switch key {
		case "price":
			parts := strings.SplitN(val, "-", 2)
			if len(parts) != 2 {
				return f, fmt.Errorf("price debe ser MIN-MAX (ej 0-1000000, 500000-, -1000000)")
			}
			if parts[0] != "" {
				v, err := strconv.ParseFloat(parts[0], 64)
				if err != nil {
					return f, fmt.Errorf("price min invalido: %v", err)
				}
				f.priceMin = v
				f.hasPriceMin = true
			}
			if parts[1] != "" {
				v, err := strconv.ParseFloat(parts[1], 64)
				if err != nil {
					return f, fmt.Errorf("price max invalido: %v", err)
				}
				f.priceMax = v
				f.hasPriceMax = true
			}
		case "condition":
			if val != "new" && val != "used" {
				return f, fmt.Errorf("condition debe ser new o used (no %q)", val)
			}
			f.condition = val
		case "shipping_cost":
			if val != "free" {
				return f, fmt.Errorf("shipping_cost solo soporta 'free' (no %q)", val)
			}
			f.freeShipping = true
		case "seller":
			if _, err := strconv.ParseInt(val, 10, 64); err != nil {
				return f, fmt.Errorf("seller debe ser numerico: %v", err)
			}
			f.sellerID = val
		case "currency":
			if val != "ARS" && val != "USD" {
				return f, fmt.Errorf("currency debe ser ARS o USD (no %q)", val)
			}
			f.currency = val
		}
	}
	return f, nil
}

type catalogProduct struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	DomainID string `json:"domain_id"`
}

func extractCatalogProducts(raw json.RawMessage) ([]catalogProduct, error) {
	// /products/search response shape: {"results": [{id, name, domain_id, ...}, ...], ...}
	var envelope struct {
		Results []catalogProduct `json:"results"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Results != nil {
		return envelope.Results, nil
	}
	// Defensive: some upstream paths return a bare array.
	var bare []catalogProduct
	if err := json.Unmarshal(raw, &bare); err == nil {
		return bare, nil
	}
	return nil, fmt.Errorf("unrecognized catalog search response shape")
}

func filterNoiseProducts(in []catalogProduct) []catalogProduct {
	out := make([]catalogProduct, 0, len(in))
	for _, p := range in {
		drop := false
		for _, fragment := range noiseDomainFragments {
			if strings.Contains(p.DomainID, fragment) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, p)
		}
	}
	return out
}

// rawItem matches a subset of the /products/{id}/items response item shape.
// ML returns the listing id under "item_id" (not "id") on this endpoint;
// "id" is captured as a fallback for other shapes (e.g. /items/{id}).
type rawItem struct {
	ItemID       string  `json:"item_id"`
	ID           string  `json:"id"`
	Price        float64 `json:"price"`
	CurrencyID   string  `json:"currency_id"`
	Condition    string  `json:"condition"`
	SellerID     int64   `json:"seller_id"`
	SoldQuantity int64   `json:"sold_quantity"`
	Permalink    string  `json:"permalink"`
	Shipping     struct {
		FreeShipping bool `json:"free_shipping"`
	} `json:"shipping"`
}

func (r rawItem) resolvedID() string {
	if r.ItemID != "" {
		return r.ItemID
	}
	return r.ID
}

func fanOutToItems(ctx context.Context, c *client.Client, products []catalogProduct, siteID string) []listing {
	sem := make(chan struct{}, itemsFanoutConc)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var out []listing

	for _, prod := range products {
		wg.Add(1)
		sem <- struct{}{}
		go func(p catalogProduct) {
			defer wg.Done()
			defer func() { <-sem }()
			path := "/products/" + p.ID + "/items"
			raw, err := c.Get(ctx, path, map[string]string{"limit": "20"})
			if err != nil {
				// Best-effort fan-out: log to stderr but don't kill the whole search.
				fmt.Fprintf(os.Stderr, "[ml-cli] warn: %s items fetch failed: %v\n", p.ID, err)
				return
			}
			items := extractItemsFromProduct(raw)
			converted := make([]listing, 0, len(items))
			for _, it := range items {
				id := it.resolvedID()
				url := it.Permalink
				if url == "" {
					url = mlPermalink(siteID, id)
				}
				converted = append(converted, listing{
					ItemID:       id,
					CatalogName:  p.Name,
					Price:        it.Price,
					Currency:     it.CurrencyID,
					Condition:    it.Condition,
					FreeShipping: it.Shipping.FreeShipping,
					SellerID:     it.SellerID,
					URL:          url,
					SoldQuantity: it.SoldQuantity,
				})
			}
			mu.Lock()
			out = append(out, converted...)
			mu.Unlock()
		}(prod)
	}
	wg.Wait()
	return out
}

func extractItemsFromProduct(raw json.RawMessage) []rawItem {
	// /products/{id}/items response shape: {"results": [...]} OR sometimes bare array.
	var env struct {
		Results []rawItem `json:"results"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && env.Results != nil {
		return env.Results
	}
	var bare []rawItem
	if err := json.Unmarshal(raw, &bare); err == nil {
		return bare
	}
	return nil
}

func applyFilters(in []listing, f searchFilter) []listing {
	out := make([]listing, 0, len(in))
	for _, l := range in {
		if f.hasPriceMin && l.Price < f.priceMin {
			continue
		}
		if f.hasPriceMax && l.Price > f.priceMax {
			continue
		}
		if f.condition != "" && l.Condition != f.condition {
			continue
		}
		if f.freeShipping && !l.FreeShipping {
			continue
		}
		if f.sellerID != "" && strconv.FormatInt(l.SellerID, 10) != f.sellerID {
			continue
		}
		if f.currency != "" && l.Currency != f.currency {
			continue
		}
		out = append(out, l)
	}
	return out
}

func sortListings(in []listing, mode string) {
	switch mode {
	case "price_asc":
		sort.SliceStable(in, func(i, j int) bool { return in[i].Price < in[j].Price })
	case "price_desc":
		sort.SliceStable(in, func(i, j int) bool { return in[i].Price > in[j].Price })
	case "sold_desc":
		sort.SliceStable(in, func(i, j int) bool { return in[i].SoldQuantity > in[j].SoldQuantity })
	case "relevance":
		// Preserve API order (catalog search rank).
	}
}

func emitSearchResults(cmd *cobra.Command, flags *rootFlags, results []listing, q, siteID, sortMode string, filtersApplied []string, totalBefore int) error {
	w := cmd.OutOrStdout()

	if flags.asJSON {
		envelope := map[string]any{
			"query":              q,
			"site_id":            siteID,
			"filters_applied":    filtersApplied,
			"sort":               sortMode,
			"total_before_filter": totalBefore,
			"total_after_filter":  len(results),
			"results":            results,
		}
		return printJSONFiltered(w, envelope, flags)
	}

	if flags.compact {
		for _, l := range results {
			fmt.Fprintf(w, "%s\t%.2f\t%s\n", l.ItemID, l.Price, l.URL)
		}
		return nil
	}

	if flags.plain {
		fmt.Fprintln(w, "id\tprice\tcurrency\tcondition\tfree_shipping\tvariant\turl\tsold_quantity\tseller_id")
		for _, l := range results {
			fmt.Fprintf(w, "%s\t%.2f\t%s\t%s\t%v\t%s\t%s\t%d\t%d\n",
				l.ItemID, l.Price, l.Currency, l.Condition, l.FreeShipping, l.CatalogName, l.URL, l.SoldQuantity, l.SellerID)
		}
		return nil
	}

	// Default human table.
	headers := []string{"#", "precio", "variante", "envío", "condición", "url"}
	rows := make([][]string, 0, len(results))
	for i, l := range results {
		shipping := "pago"
		if l.FreeShipping {
			shipping = "gratis"
		}
		rows = append(rows, []string{
			strconv.Itoa(i + 1),
			fmt.Sprintf("%s %.2f", l.Currency, l.Price),
			truncateStr(l.CatalogName, 50),
			shipping,
			l.Condition,
			l.URL,
		})
	}
	return flags.printTable(cmd, headers, rows)
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
