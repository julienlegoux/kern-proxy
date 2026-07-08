package main

// Ports: packages/ai/src/cli.ts's main()'s help branch, expanded with the
// "help" command name itself and updated for `list`'s broader scope here
// (the embedded catalog, not just the 3 OAuth providers — see list.go).

import "io"

// helpText documents the CLI's subcommands.
const helpText = `Usage: pi-ai <command> [args]

Commands:
  login [provider]  Log in to an OAuth provider (interactive if omitted)
  list              List built-in providers and their models
  help              Show this help message

Examples:
  pi-ai login                  interactive provider selection
  pi-ai login anthropic        log in to a specific provider
  pi-ai list                   list providers and models
`

// runHelp writes the CLI's usage text to w.
func runHelp(w io.Writer) error {
	_, err := io.WriteString(w, helpText)
	return err
}
