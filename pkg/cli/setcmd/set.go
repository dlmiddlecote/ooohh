package setcmd

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"

	"github.com/dlmiddlecote/ooohh/pkg/cli/rootcmd"
)

// Config for the set subcommand, including a reference to the ooohh service.
type Config struct {
	rootConfig *rootcmd.Config
	stdout     io.Writer
}

// New returns a usable ffcli.Command for the set subcommand.
func New(rootConfig *rootcmd.Config, stdout io.Writer) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
		stdout:     stdout,
	}

	fs := flag.NewFlagSet("ooohh set", flag.ExitOnError)
	rootConfig.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "ooohh set [flags] <dial id> <token>",
		ShortHelp:  "Sets dial to use",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	if n := len(args); n != 2 {
		return errors.New(fmt.Sprintf("set requires 2 arguments, but you provided %d", n))
	}

	err := c.rootConfig.Save(args[0], args[1])
	if err != nil {
		return errors.Wrap(err, "updating config")
	}

	fmt.Fprint(c.stdout, "Config updated.\n")

	return nil
}
