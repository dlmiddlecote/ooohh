package querycmd

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"

	"github.com/dlmiddlecote/ooohh/pkg/cli/rootcmd"
)

// Config for the query subcommand, including a reference to the ooohh service.
type Config struct {
	rootConfig *rootcmd.Config
	stdout     io.Writer
}

// New returns a usable ffcli.Command for the query subcommand.
func New(rootConfig *rootcmd.Config, stdout io.Writer) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
		stdout:     stdout,
	}

	fs := flag.NewFlagSet("ooohh ?", flag.ExitOnError)
	rootConfig.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "?",
		ShortUsage: "ooohh ? [flags]",
		ShortHelp:  "Gets dial information",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	if n := len(args); n != 0 {
		return errors.New(fmt.Sprintf("? requires 0 arguments, but you provided %d", n))
	}

	d, err := c.rootConfig.Service.GetDial(ctx, c.rootConfig.DialID)
	if err != nil {
		return errors.Wrap(err, "retrieving dial")
	}

	fmt.Fprintf(c.stdout, "Your dial (%s) is set to %.1f.\n", d.ID, d.Value)

	return nil
}
