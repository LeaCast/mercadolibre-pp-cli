// Copyright 2026 leandrodc. Licensed under Apache-2.0. See LICENSE.
//
// MANUAL PATCH — see .printing-press-patches.json id="items-search-workflow".
//
// Original generator output: a top-level `items <item_id>` command that did
// a single GET /items/{item_id}. That has been restructured into a parent
// `items` command with two subcommands: `items get <item_id>` (preserves
// the old behavior) and `items search` (the catalog→items workflow).
//
// Backward-compat note: invoking `items <id>` directly no longer works —
// users must say `items get <id>`. Documented in CHANGELOG/patches.json.

package cli

import (
	"github.com/spf13/cobra"
)

// newItemsPromotedCmd returns the items parent command. Name kept (instead
// of renaming to newItemsCmd) so the registration site in root.go doesn't
// need a structural change — only the body of this function changed.
func newItemsPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "items",
		Short: "Operaciones sobre publicaciones (get por id, search workflow catalog→items)",
		Long:  "Operaciones sobre publicaciones de MercadoLibre. Subcomandos: get (detalle por item_id), search (workflow keyword→listings con filtros).",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newItemsGetCmd(flags))
	cmd.AddCommand(newItemsSearchCmd(flags))

	return cmd
}
