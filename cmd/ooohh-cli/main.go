package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/blendle/zapdriver"
	"github.com/dlmiddlecote/ooohh/pkg/client"
	"github.com/mitchellh/go-homedir"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type dialCache struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stdout, "error: %v", err)
		os.Exit(1)
	}
}

func run(a []string, stdout, stderr io.Writer) error {

	home, err := homedir.Dir()
	if err != nil {
		return errors.Wrap(err, "could not parse home directory")
	}

	var (
		rootFlagSet = flag.NewFlagSet("ooohh", flag.ExitOnError)
		u           = rootFlagSet.String("url", "https://ooohh.wtf", "set base url")
		cacheDir    = rootFlagSet.String("cache", filepath.Join(home, ".ooohh"), "cache dir location")
	)

	// create cache dir
	if _, err := os.Stat(*cacheDir); os.IsNotExist(err) {
		err = os.MkdirAll(*cacheDir, os.ModePerm)
		if err != nil {
			return errors.Wrap(err, "could not create cache directory")
		}
	}

	// create cache file
	cacheFilePath := filepath.Join(*cacheDir, "ooohh.json")
	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
		f, err := os.OpenFile(cacheFilePath, os.O_RDONLY|os.O_CREATE, 0644)
		if err != nil {
			return errors.Wrap(err, "could not create cache file")
		}
		f.Close()
	}

	base, err := url.Parse(*u)
	if err != nil {
		return errors.Wrap(err, "invalid url")
	}

	//
	// Logging
	//

	var logger *zap.SugaredLogger
	{
		if l, err := zapdriver.NewProduction(); err != nil {
			return errors.Wrap(err, "creating logger")
		} else {
			logger = l.Sugar()
		}
	}
	// Flush logs at the end of the applications lifetime
	defer logger.Sync()

	s := client.NewClient(base, logger)

	create := &ffcli.Command{
		Name:       "create",
		ShortUsage: "ooohh create <name> <token>",
		ShortHelp:  "Creates a new dial.",
		Exec: func(ctx context.Context, args []string) error {
			if n := len(args); n != 2 {
				return errors.New(fmt.Sprintf("create requires 2 arguments, but you provided %d\n", n))
			}

			d, err := s.CreateDial(ctx, args[0], args[1])
			if err != nil {
				return errors.Wrap(err, "creating dial")
			}

			fmt.Fprintf(stdout, "created dial %s (%s)\n", d.Name, d.ID)

			_ = dialCache{
				ID:    string(d.ID),
				Token: args[1],
			}

			// store cache

			return nil
		},
	}

	root := &ffcli.Command{
		ShortUsage:  "ooohh [flags] <subcommand>",
		FlagSet:     rootFlagSet,
		Subcommands: []*ffcli.Command{create},
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}

	return root.ParseAndRun(context.Background(), a)
}
