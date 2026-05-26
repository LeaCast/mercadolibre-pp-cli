// Copyright 2026 leandrodc. Licensed under Apache-2.0. See LICENSE.
//
// MANUAL PATCH — see .printing-press-patches.json id="items-search-workflow".
// Hosts the `items get <item_id>` subcommand. Logic moved verbatim from the
// previous top-level `items <item_id>` command (promoted_items.go) when
// `items` was restructured into a parent with get + search children.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newItemsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <item_id>",
		Short:       "Detalle completo de una publicacion (precio, stock, fotos, descripcion, vendedor)",
		Long:        "Detalle completo de una publicacion (precio, stock, fotos, descripcion, vendedor)",
		Example:     "  mercadolibre-pp-cli items get MLA1234567890",
		Annotations: map[string]string{"pp:endpoint": "items.get", "pp:method": "GET", "pp:path": "/items/{item_id}", "mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/items/{item_id}", "item_id", args[0])
			params := map[string]string{}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "items", false, path, params, nil, cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			data = extractResponseData(data)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var countItems []json.RawMessage
				if json.Unmarshal(data, &countItems) != nil {
					countItems = []json.RawMessage{data}
				}
				printProvenance(cmd, len(countItems), prov)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}
