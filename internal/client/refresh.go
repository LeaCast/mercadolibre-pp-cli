// Copyright 2026 leandrodc. Licensed under Apache-2.0. See LICENSE.
//
// MANUAL PATCH — see .printing-press-patches.json id="transparent-token-refresh".
// MercadoLibre access tokens live 6h. Without transparent refresh, every
// command issued >5h59m after the last refresh fails 401 and the user
// needs an external cron job. This helper is called by Client.authHeader
// when the cached expiry is within 30 minutes, BEFORE building the request.
// It uses stdlib net/http (NOT the surf client used by Client.do) to avoid
// re-entrant calls through the refresh-trigger path.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"mercadolibre-pp-cli/internal/config"
)

const tokenEndpoint = "https://api.mercadolibre.com/oauth/token"

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	UserID       int64  `json:"user_id"`
}

// refreshAccessToken exchanges the refresh_token for a new access_token and
// persists the result to disk via cfg.SaveTokens. Returns nil on success;
// the caller (Client.authHeader) updates its in-memory header by reading
// cfg.AccessToken after this returns.
//
// 15s timeout: deliberately tighter than ConfiguredTimeout because the
// refresh blocks every outbound API call. If the token endpoint hangs we'd
// rather fall through to "use the current token and let it 401" than
// deadlock the user's CLI.
func refreshAccessToken(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("refresh: nil config")
	}
	if cfg.RefreshToken == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return fmt.Errorf("refresh: missing refresh_token, client_id, or client_secret")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)
	form.Set("refresh_token", cfg.RefreshToken)

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "POST", tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("refresh: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("refresh: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("refresh: reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh: HTTP %d: %s", resp.StatusCode, truncateBody(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return fmt.Errorf("refresh: parsing response: %w", err)
	}
	if tok.AccessToken == "" {
		return fmt.Errorf("refresh: empty access_token in response")
	}

	// ML returns a new refresh_token on each refresh (rotating refresh tokens).
	// Fall back to the existing one only if the server omitted it.
	newRefresh := tok.RefreshToken
	if newRefresh == "" {
		newRefresh = cfg.RefreshToken
	}
	expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)

	if err := cfg.SaveTokens(cfg.ClientID, cfg.ClientSecret, tok.AccessToken, newRefresh, expiry); err != nil {
		return fmt.Errorf("refresh: persisting tokens: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[ml-cli] token refreshed (valid until %s)\n", expiry.Format("15:04:05"))
	return nil
}
