package main

// Ports: packages/ai/src/cli.ts's "list" branch of main(), broadened per
// this issue's scope from the 3 OAuth login targets to the full embedded
// catalog (ai/providers, Epic 11) — see runList's doc comment below.

import (
	"fmt"
	"io"
	"sort"

	"github.com/julienlegoux/kern-proxy/ai"
)

// runList writes every provider's id, display name, and known models to w,
// sorted by provider id then model id for stable, diffable output.
//
// Ports: packages/ai/src/cli.ts's "list" branch, which only enumerated the 3
// OAuth login targets (getOAuthProviders()). This port's `list` is
// deliberately broader per this issue's scope: it surfaces the full embedded
// catalog wired into every built-in provider binding (ai/providers, Epic 11)
// rather than just the OAuth subset — the narrower OAuth list is still
// available through `login`'s interactive provider prompt (oauth.go).
func runList(w io.Writer, providers []ai.Provider) error {
	sorted := append([]ai.Provider(nil), providers...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID() < sorted[j].ID() })

	for _, p := range sorted {
		models := append([]*ai.Model(nil), p.GetModels()...)
		sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })

		if _, err := fmt.Fprintf(w, "%s (%s) - %d model(s)\n", p.ID(), p.Name(), len(models)); err != nil {
			return err
		}
		for _, m := range models {
			if _, err := fmt.Fprintf(w, "  %-40s %s\n", m.ID, m.Name); err != nil {
				return err
			}
		}
	}
	return nil
}
