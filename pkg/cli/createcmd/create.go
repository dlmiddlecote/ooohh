package createcmd

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"

	"github.com/dlmiddlecote/ooohh/pkg/cli/rootcmd"
)

// Config for the create subcommand, including a reference to the ooohh service.
type Config struct {
	rootConfig *rootcmd.Config
	stdout     io.Writer
}

// New returns a usable ffcli.Command for the create subcommand.
func New(rootConfig *rootcmd.Config, stdout io.Writer) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
		stdout:     stdout,
	}

	fs := flag.NewFlagSet("ooohh create", flag.ExitOnError)
	rootConfig.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "ooohh create [flags] <name> <token>",
		ShortHelp:  "Creates a new dial",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	if n := len(args); n != 2 {
		return errors.New(fmt.Sprintf("create requires 2 arguments, but you provided %d", n))
	}

	d, err := c.rootConfig.Service.CreateDial(ctx, args[0], args[1])
	if err != nil {
		return errors.Wrap(err, "creating dial")
	}

	// store cache
	err = c.rootConfig.Save(string(d.ID), args[1])
	if err != nil {
		return errors.Wrap(err, "storing config")
	}

	fmt.Fprintf(c.stdout, "created dial %s (%s)\n", d.Name, d.ID)

	return nil
}
