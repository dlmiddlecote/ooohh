package rootcmd

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/errors"

	"github.com/dlmiddlecote/ooohh"
)

// Config for the root command, including flags and types that should be
// available to each subcommand.
type Config struct {
	DialID  ooohh.DialID
	Token   string
	Service ooohh.Service
	URL     string

	home      string
	cacheDir  string
	cacheFile string
}

// New constructs a usable ffcli.Command and an empty Config. The caller must
// initialize the config's ooohh service field, dial id and token.
func New(home string) (*ffcli.Command, *Config) {
	cfg := Config{home: home}

	fs := flag.NewFlagSet("ooohh", flag.ExitOnError)
	cfg.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "ooohh",
		ShortUsage: "ooohh [flags] <subcommand> [flags] [<arg>...]",
		FlagSet:    fs,
		Exec:       cfg.Exec,
	}, &cfg
}

// RegisterFlags registers the flag fields into the provided flag.FlagSet. This
// helper function allows subcommands to register the root flags into their
// flagsets, creating "global" flags that can be passed after any subcommand at
// the commandline.
func (c *Config) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.URL, "url", "https://ooohh.wtf", "ooohh service base url")
	fs.StringVar(&c.cacheDir, "cache", filepath.Join(c.home, ".ooohh"), "cache dir location")
}

// Exec function for this command.
func (c *Config) Exec(context.Context, []string) error {
	// The root command has no meaning, so if it gets executed,
	// display the usage text to the user instead.
	return flag.ErrHelp
}

// InitFromCache creates the cache file if it doesn't exist,
// or reads configuration from the cache file, and sets the values on itself.
func (c *Config) InitFromCache() error {

	c.cacheFile = filepath.Join(c.cacheDir, "ooohh.json")

	// check if cache file exists.
	if _, err := os.Stat(c.cacheFile); os.IsNotExist(err) {
		// create cache dir if it doesn't exist.
		if _, err := os.Stat(c.cacheDir); os.IsNotExist(err) {
			err = os.MkdirAll(c.cacheDir, os.ModePerm)
			if err != nil {
				return errors.Wrap(err, "could not create cache directory")
			}
		}

		// create cache file.
		f, err := os.OpenFile(c.cacheFile, os.O_RDONLY|os.O_CREATE, 0600)
		if err != nil {
			return errors.Wrap(err, "could not create cache file")
		}
		f.Close()

	} else {

		// Read from cache file.
		b, err := ioutil.ReadFile(c.cacheFile)
		if err != nil {
			return errors.Wrap(err, "reading file")
		}

		// Parse cache file.
		var cc struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		}
		err = json.Unmarshal(b, &cc)
		if err != nil {
			return errors.Wrap(err, "parsing file")
		}

		// Set config.
		c.DialID = ooohh.DialID(cc.ID)
		c.Token = cc.Token
	}

	return nil
}

// Save writes the config to the cache file.
func (c *Config) Save(dialID, token string) error {
	cc := struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}{
		ID:    dialID,
		Token: token,
	}

	b, err := json.Marshal(cc)
	if err != nil {
		return errors.Wrap(err, "marshalling to json")
	}

	err = ioutil.WriteFile(c.cacheFile, b, 0600)
	if err != nil {
		return errors.Wrap(err, "writing to file")
	}
	return nil
}
