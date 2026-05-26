// Copyright 2026 leandrodc. Licensed under Apache-2.0. See LICENSE.
//
// MANUAL PATCH — shared by auth_login.go (OAuth authorize URL) and
// items_search.go (permalink construction). Hardcoded mapping for the 8
// MercadoLibre country sites; unknown sites fall back to .com.ar so the
// CLI degrades to MLA behavior instead of producing an invalid hostname.

package cli

// mlSiteDomain returns the country TLD suffix (e.g. ".com.ar") for a given
// MercadoLibre site_id. Returns ".com.ar" for unknown sites — defensive
// default rather than empty string, since both the auth domain and the
// permalink construction assume a valid TLD downstream.
func mlSiteDomain(siteID string) string {
	switch siteID {
	case "MLA":
		return ".com.ar"
	case "MLB":
		return ".com.br"
	case "MLM":
		return ".com.mx"
	case "MLC":
		return ".cl"
	case "MCO":
		return ".com.co"
	case "MLU":
		return ".com.uy"
	case "MPE":
		return ".com.pe"
	case "MLV":
		return ".com.ve"
	default:
		return ".com.ar"
	}
}

// mlPermalink constructs a public ML article URL from a site_id + item_id
// (e.g. "MLA1234567890" -> "https://articulo.mercadolibre.com.ar/MLA-1234567890").
// The wire format puts a hyphen between the site code and the numeric id;
// the official permalink returned by the API uses the same shape. When the
// item_id doesn't begin with the site code (defensive), we return the
// item_id-only form which still resolves on the catalog redirect path.
func mlPermalink(siteID, itemID string) string {
	domain := mlSiteDomain(siteID)
	// Item IDs come in like "MLA1234567890"; split into prefix + numeric.
	if len(itemID) > len(siteID) && itemID[:len(siteID)] == siteID {
		return "https://articulo.mercadolibre" + domain + "/" + siteID + "-" + itemID[len(siteID):]
	}
	return "https://articulo.mercadolibre" + domain + "/" + itemID
}
