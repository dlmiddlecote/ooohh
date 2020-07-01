package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/blendle/zapdriver"
	"github.com/dlmiddlecote/ooohh"
	"github.com/dlmiddlecote/ooohh/pkg/client"
	"github.com/mitchellh/go-homedir"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type cache struct {
	f string
}

func (c cache) GetConfig() (string, string, error) {
	b, err := ioutil.ReadFile(c.f)
	if err != nil {
		return "", "", errors.Wrap(err, "reading file")
	}

	var cc struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	err = json.Unmarshal(b, &cc)
	if err != nil {
		return "", "", errors.Wrap(err, "parsing file")
	}

	return cc.ID, cc.Token, nil
}

func (c cache) SetConfig(id, token string) error {
	cc := struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}{
		ID:    id,
		Token: token,
	}

	b, err := json.Marshal(cc)
	if err != nil {
		return errors.Wrap(err, "marshalling to json")
	}

	err = ioutil.WriteFile(c.f, b, 0644)
	if err != nil {
		return errors.Wrap(err, "writing to file")
	}
	return nil
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

	c := cache{cacheFilePath}
	id, token, _ := c.GetConfig()

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
	defer logger.Sync() //nolint:errcheck

	s := client.NewClient(base, logger)

	create := &ffcli.Command{
		Name:       "create",
		ShortUsage: "ooohh create <name> <token>",
		ShortHelp:  "Creates a new dial",
		Exec: func(ctx context.Context, args []string) error {
			if n := len(args); n != 2 {
				return errors.New(fmt.Sprintf("create requires 2 arguments, but you provided %d\n", n))
			}

			d, err := s.CreateDial(ctx, args[0], args[1])
			if err != nil {
				return errors.Wrap(err, "creating dial")
			}

			fmt.Fprintf(stdout, "created dial %s (%s)\n", d.Name, d.ID)

			// store cache
			err = c.SetConfig(string(d.ID), args[1])
			if err != nil {
				return errors.Wrap(err, "storing config")
			}

			return nil
		},
	}

	wtf := &ffcli.Command{
		Name:       "wtf",
		ShortUsage: "ooohhh wtf <value>",
		ShortHelp:  "Updates dial value",
		Exec: func(ctx context.Context, args []string) error {
			if n := len(args); n != 1 {
				return errors.New(fmt.Sprintf("wtf requires 1 argument, but you provided %d\n", n))
			}

			value, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return errors.Wrapf(err, "value must be a number, you provided %s\n", args[0])
			}

			err = s.SetDial(ctx, ooohh.DialID(id), token, value)
			if err != nil {
				return errors.Wrap(err, "setting dial")
			}

			fmt.Fprint(stdout, "wtf level set 💥\n")

			return nil
		},
	}

	root := &ffcli.Command{
		ShortUsage:  "ooohh [flags] <subcommand>",
		FlagSet:     rootFlagSet,
		Subcommands: []*ffcli.Command{create, wtf},
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}

	return root.ParseAndRun(context.Background(), a)
}
