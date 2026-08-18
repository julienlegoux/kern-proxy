// Command pi-ai is the user-facing CLI for kern-link's OAuth login and
// built-in provider catalog.
//
// Ports: packages/ai/src/cli.ts's main() argv dispatch (login/list/help).
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kern-ia/kern-link/ai/auth"
	"github.com/kern-ia/kern-link/ai/providers"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run dispatches one CLI invocation and returns its process exit code.
// login reads interactive input from os.Stdin and persists credentials to
// the default ~/.pi/agent/auth.json store (ai/auth.FileCredentialStore);
// list and help are pure functions of the embedded catalog and need neither.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_ = runHelp(stdout)
		return 0
	}

	switch args[0] {
	case "help", "--help", "-h":
		_ = runHelp(stdout)
		return 0

	case "list":
		if err := runList(stdout, providers.Providers()); err != nil {
			fmt.Fprintln(stderr, "Error:", err)
			return 1
		}
		return 0

	case "login":
		store, err := defaultCredentialStore()
		if err != nil {
			fmt.Fprintln(stderr, "Error:", err)
			return 1
		}
		if err := runLogin(context.Background(), args[1:], os.Stdin, stdout, store, oauthProviders()); err != nil {
			fmt.Fprintln(stderr, "Error:", err)
			return 1
		}
		return 0

	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n", args[0])
		fmt.Fprintln(stderr, "Use 'pi-ai help' for usage")
		return 1
	}
}

// defaultCredentialStore opens the conventional ~/.pi/agent/auth.json
// credential store (ai/auth.FileCredentialStore, Epic 3).
func defaultCredentialStore() (*auth.FileCredentialStore, error) {
	path, err := auth.DefaultPath()
	if err != nil {
		return nil, err
	}
	return auth.NewFileCredentialStore(path), nil
}
