package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"

	"github.com/dlmiddlecote/ooohh/pkg/cli/createcmd"
	"github.com/dlmiddlecote/ooohh/pkg/cli/rootcmd"
	"github.com/dlmiddlecote/ooohh/pkg/cli/wtfcmd"
	"github.com/dlmiddlecote/ooohh/pkg/client"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stdout, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {

	// Find HOME directory.
	home, err := homedir.Dir()
	if err != nil {
		return errors.Wrap(err, "could not parse home directory")
	}

	//
	// Command-line options.
	//

	var (
		rootCommand, rootConfig = rootcmd.New(home)
		createCommand           = createcmd.New(rootConfig, stdout)
		wtfCommand              = wtfcmd.New(rootConfig, stdout)
	)

	// Register subcommands.
	rootCommand.Subcommands = []*ffcli.Command{
		createCommand,
		wtfCommand,
	}

	// Parse arguments.
	if err := rootCommand.Parse(args); err != nil {
		return errors.Wrap(err, "parsing arguments")
	}

	// Parse base URL, check it is valid.
	base, err := url.Parse(rootConfig.URL)
	if err != nil {
		return errors.Wrap(err, "invalid url")
	}

	// Create client to ooohh.
	s := client.NewClient(base)

	// Register ooohh client with root.
	rootConfig.Service = s

	// Register cached config with root.
	err = rootConfig.InitFromCache()
	if err != nil {
		return errors.Wrap(err, "initializing cached config")
	}

	// Run any commands.
	return rootCommand.Run(context.Background())
}
