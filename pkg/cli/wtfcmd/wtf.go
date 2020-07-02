package wtfcmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/dlmiddlecote/ooohh/pkg/cli/rootcmd"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
)

// Config for the wtf subcommand, including a reference to the ooohh service.
type Config struct {
	rootConfig *rootcmd.Config
	stdout     io.Writer
}

// New returns a usable ffcli.Command for the wtf subcommand.
func New(rootConfig *rootcmd.Config, stdout io.Writer) *ffcli.Command {
	cfg := Config{
		rootConfig: rootConfig,
		stdout:     stdout,
	}

	fs := flag.NewFlagSet("ooohh wtf", flag.ExitOnError)
	rootConfig.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "wtf",
		ShortUsage: "ooohh wtf [flags] <value>",
		ShortHelp:  "Updates dial value",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}
}

// Exec function for this command.
func (c *Config) Exec(ctx context.Context, args []string) error {
	if n := len(args); n != 1 {
		return errors.New(fmt.Sprintf("wtf requires 1 argument, but you provided %d", n))
	}

	value, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return errors.Wrapf(err, "value must be a number, you provided %s", args[0])
	}

	err = c.rootConfig.Service.SetDial(ctx, c.rootConfig.DialID, c.rootConfig.Token, value)
	if err != nil {
		return errors.Wrap(err, "setting dial")
	}

	fmt.Fprint(c.stdout, "wtf level set 💥\n")

	return nil
}
