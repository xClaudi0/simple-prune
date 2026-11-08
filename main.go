// Command docker-simpleprune is a Docker CLI plugin (`docker simpleprune`,
// also installable as `docker sp`). It shows which images
// `docker image prune [-a]` would remove, without removing anything.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/cli/cli-plugins/metadata"
	"github.com/docker/cli/cli-plugins/plugin"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"
	"github.com/xClaudi0/simple-prune/internal/prune"
)

var version = "0.1.0"

func main() {
	plugin.Run(newCommand, metadata.Metadata{
		SchemaVersion:    "0.1.0",
		Vendor:           "Claudio Bellanti",
		Version:          version,
		ShortDescription: "Preview images removed by 'docker image prune [-a]' (read-only)",
	})
}

// commandName returns the plugin command name, taken from the executable name:
// docker-sp -> "sp", docker-simpleprune -> "simpleprune". The Docker CLI runs
// plugins as `docker-<name> <name> ...` and requires <name> to match
// ^[a-z][a-z0-9]*$ (so "simple-prune" is not allowed), hence these two names.
func commandName() string {
	base := filepath.Base(os.Args[0])
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if name := strings.TrimPrefix(base, "docker-"); name == "sp" {
		return name
	}
	return "simpleprune"
}

func newCommand(dockerCLI command.Cli) *cobra.Command {
	var all, asJSON bool
	var rawFilters []string
	cmd := &cobra.Command{
		Use:   commandName() + " [-a|--all] [--filter key=value] [--json]",
		Short: "Show which images 'docker image prune [-a]' would remove, without removing anything",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageError(cmd, fmt.Errorf("unknown argument: %q", args[0]))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			filters, err := prune.ParseFilters(rawFilters)
			if err != nil {
				return usageError(cmd, err)
			}
			res, err := prune.Select(cmd.Context(), prune.NewDaemon(dockerCLI.Client()), all, filters...)
			if err != nil {
				return err
			}
			if asJSON {
				return prune.WriteJSON(dockerCLI.Out(), res)
			}
			return prune.WriteTable(dockerCLI.Out(), res)
		},
	}
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error { return usageError(c, err) })
	f := cmd.Flags()
	f.BoolVarP(&all, "all", "a", false, "Simulate 'docker image prune -a' (all unused images, not only dangling)")
	f.BoolVar(&asJSON, "json", false, "Output JSON instead of a table")
	f.StringArrayVar(&rawFilters, "filter", nil, "Filter like 'docker image prune --filter': until=<duration|timestamp>, label=<key>[=<value>], label!=<key>[=<value>] (repeatable)")
	return cmd
}

// usageError prints the help to stderr and returns err (the plugin framework
// prints err and exits with status 1).
func usageError(cmd *cobra.Command, err error) error {
	fmt.Fprintln(cmd.ErrOrStderr(), cmd.UsageString())
	return err
}
