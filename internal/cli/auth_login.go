// Copyright 2026 leandrodc. Licensed under Apache-2.0. See LICENSE.
//
// MANUAL PATCH — see .printing-press-patches.json id="auth-login-wizard".
//
// Interactive OAuth dance for MercadoLibre.
//
// Why not auto-capture the code with a local HTTP server: MercadoLibre's
// app config enforces https:// on registered Redirect URIs and rejects
// http://localhost outright. A self-signed HTTPS listener works but
// produces a browser warning users have to dismiss, which is worse UX
// than a single paste. So we use a public HTTPS bouncer (httpbin.org/get
// by default — any HTTPS echo service works) and prompt the user to
// paste the `code` shown by the bouncer. Everything else is automated:
//   1. wizard opens the auth URL in the default browser
//   2. user authorizes, the bouncer page shows ?code=... in its echo
//   3. user pastes the code into the terminal
//   4. wizard exchanges code → tokens and persists everything
//   5. transparent refresh in client.maybeRefresh keeps the session alive
//      indefinitely

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"mercadolibre-pp-cli/internal/config"
)

const (
	defaultLoginSiteID      = "MLA"
	defaultLoginRedirectURI = "https://httpbin.org/get"
)

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSiteID      string
		flagRedirectURI string
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Wizard interactivo de OAuth (abre navegador, pegás el code, guarda tokens)",
		Long: `Completa el flujo OAuth de MercadoLibre en un solo comando:

  1. Si no tenés app_id/secret guardados, te pide crearlos en devcenter.
  2. Abre el navegador en la URL de autorización de MercadoLibre.
  3. Después de que autorices, el redirect te muestra un JSON con el code.
  4. Pegás el code en la terminal cuando el wizard te lo pide.
  5. El wizard hace el intercambio code→tokens y los guarda.
  6. De ahí en adelante el refresh es transparente (sin cron, sin scripts).

Por qué no es 100% automático: MercadoLibre rechaza http://localhost en
Redirect URIs (exige https://). Usar HTTPS local requeriría certificado
autofirmado y un warning de seguridad cada vez. Más fácil: usar httpbin.org
como espejo y pegar el code una vez por sesión (cada 6 meses si el refresh
token sigue válido).

Requiere modo interactivo. Para CI / scripts usá 'auth set-token' con un
token preobtenido.`,
		Example: `  mercadolibre-pp-cli auth login
  mercadolibre-pp-cli auth login --site-id MLB
  mercadolibre-pp-cli auth login --redirect-uri https://mi-bouncer.com/echo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.noInput {
				return usageErr(fmt.Errorf("auth login requires interactive mode; use 'auth set-token' for scripts"))
			}
			return runAuthLogin(cmd, flags, flagSiteID, flagRedirectURI)
		},
	}
	cmd.Flags().StringVar(&flagSiteID, "site-id", defaultLoginSiteID, "Codigo del sitio ML (MLA, MLB, MLM, MLC, MCO, MLU, MPE, MLV)")
	cmd.Flags().StringVar(&flagRedirectURI, "redirect-uri", defaultLoginRedirectURI, "Redirect URI registrada en tu app ML (debe ser HTTPS)")
	return cmd
}

func runAuthLogin(cmd *cobra.Command, flags *rootFlags, siteID, redirectURI string) error {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}

	stderr := cmd.ErrOrStderr()
	reader := bufio.NewReader(os.Stdin)

	// Step 1: resolve client_id + client_secret (saved or prompt fresh).
	clientID, clientSecret, err := resolveAppCredentials(cfg, reader, stderr, redirectURI)
	if err != nil {
		return err
	}

	// Step 2: construct authorization URL.
	authURL := buildAuthURL(siteID, clientID, redirectURI)

	// Step 3: open browser, instruct user.
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "Abriendo el navegador para autorizar…")
	fmt.Fprintf(stderr, "Si no se abre solo, copiá esta URL y pegala en el browser:\n  %s\n\n", authURL)
	if err := openBrowser(authURL); err != nil {
		fmt.Fprintf(stderr, "(no pude abrir el navegador automáticamente: %v)\n\n", err)
	}

	fmt.Fprintln(stderr, "Después de autorizar, vas a ver un JSON en pantalla.")
	fmt.Fprintf(stderr, "Buscá el campo \"args.code\" (algo como TG-xxxxxxxx-yyyyyy) y pegalo abajo.\n\n")
	fmt.Fprint(stderr, "Code: ")

	codeLine, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("leyendo code: %w", err)
	}
	code := strings.TrimSpace(codeLine)
	// Tolerate users pasting the whole URL or a JSON snippet with the code.
	code = extractCode(code)
	if code == "" {
		return fmt.Errorf("code vacío; reintentá")
	}

	// Step 4: exchange code → tokens.
	tok, err := exchangeCodeForTokens(cmd.Context(), clientID, clientSecret, code, redirectURI)
	if err != nil {
		return fmt.Errorf("intercambiando code por token: %w", err)
	}

	expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if err := cfg.SaveTokens(clientID, clientSecret, tok.AccessToken, tok.RefreshToken, expiry); err != nil {
		return configErr(fmt.Errorf("guardando tokens: %w", err))
	}

	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"authenticated": true,
			"user_id":       tok.UserID,
			"expires_at":    expiry.UTC().Format(time.RFC3339),
			"scope":         tok.Scope,
		}, flags)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintln(w, green("✅ Autenticado correctamente"))
	fmt.Fprintf(w, "  User ID: %d\n", tok.UserID)
	fmt.Fprintf(w, "  Token válido por: %dh\n", tok.ExpiresIn/3600)
	fmt.Fprintln(w, "  Refresh automático: habilitado (transparente, sin cron)")
	return nil
}

// extractCode pulls the OAuth authorization code out of the user's paste.
// Accepts plain code, a full redirect URL, or a snippet of JSON, so the
// wizard is tolerant of how the user copies from the bouncer page.
func extractCode(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// If it parses as a URL with a code query param, take that.
	if u, err := url.Parse(raw); err == nil && u.Query().Get("code") != "" {
		return u.Query().Get("code")
	}
	// If it looks like JSON containing `"code": "..."`, scrape it.
	if i := strings.Index(raw, `"code"`); i >= 0 {
		rest := raw[i+len(`"code"`):]
		if j := strings.Index(rest, `"`); j >= 0 {
			rest = rest[j+1:]
			if k := strings.Index(rest, `"`); k >= 0 {
				return rest[:k]
			}
		}
	}
	// Strip surrounding quotes if present.
	return strings.Trim(raw, `"'`)
}

func resolveAppCredentials(cfg *config.Config, reader *bufio.Reader, stderr io.Writer, redirectURI string) (string, string, error) {
	if cfg.ClientID != "" && cfg.ClientSecret != "" {
		fmt.Fprintf(stderr, "Encontré credenciales de app guardadas (client_id: %s…).\n", maskShort(cfg.ClientID))
		fmt.Fprint(stderr, "¿Usar las guardadas? [Y/n]: ")
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || line == "y" || line == "yes" || line == "s" || line == "si" {
			return cfg.ClientID, cfg.ClientSecret, nil
		}
	}

	fmt.Fprintln(stderr, "Necesito el App ID y Client Secret de tu app de MercadoLibre.")
	fmt.Fprintln(stderr, "")
	fmt.Fprintln(stderr, "  1) Abrí https://developers.mercadolibre.com.ar/devcenter (te lo abro yo)")
	fmt.Fprintln(stderr, "  2) Creá una app (o abrí la existente) — nombre tipo \"mercadolibre-pp-cli\"")
	fmt.Fprintf(stderr, "  3) Agregá Redirect URI: %s\n", redirectURI)
	fmt.Fprintln(stderr, "  4) Habilitá los flows: Authorization Code + Refresh Token")
	fmt.Fprintln(stderr, "  5) Copiá App ID y Client Secret y pegalos abajo.")
	fmt.Fprintln(stderr, "")
	_ = openBrowser("https://developers.mercadolibre.com.ar/devcenter")

	fmt.Fprint(stderr, "App ID: ")
	cid, err := reader.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("leyendo App ID: %w", err)
	}
	cid = strings.TrimSpace(cid)
	if cid == "" {
		return "", "", fmt.Errorf("App ID vacío")
	}
	fmt.Fprint(stderr, "Client Secret: ")
	cs, err := reader.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("leyendo Client Secret: %w", err)
	}
	cs = strings.TrimSpace(cs)
	if cs == "" {
		return "", "", fmt.Errorf("Client Secret vacío")
	}
	return cid, cs, nil
}

func buildAuthURL(siteID, clientID, redirectURI string) string {
	domain := mlSiteDomain(siteID)
	base := "https://auth.mercadolibre" + domain + "/authorization"
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	return base + "?" + q.Encode()
}

type loginTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	UserID       int64  `json:"user_id"`
}

func exchangeCodeForTokens(ctx context.Context, clientID, clientSecret, code, redirectURI string) (*loginTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, "POST", "https://api.mercadolibre.com/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint HTTP %d: %s", resp.StatusCode, truncateTokenBody(body))
	}
	var tok loginTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parseando respuesta de token: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("respuesta sin access_token")
	}
	return &tok, nil
}

func openBrowser(rawURL string) error {
	switch runtime.GOOS {
	case "windows":
		// cmd /c start "" <url> — the empty title argument is required
		// because URLs with `&` would otherwise be parsed as the title.
		return exec.Command("cmd", "/c", "start", "", rawURL).Start()
	case "darwin":
		return exec.Command("open", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}

func maskShort(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + "…"
}

func truncateTokenBody(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}
