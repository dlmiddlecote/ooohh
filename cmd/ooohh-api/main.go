package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/blendle/zapdriver"
	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/dlmiddlecote/ooohh"
	"github.com/dlmiddlecote/ooohh/pkg/service"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stdout, "error: %v", err)
		os.Exit(1)
	}
}

func run() error {

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

	var err error
	var s ooohh.Service
	{
		db, err := bolt.Open("/tmp/bolt.db", 0600, nil)
		if err != nil {
			return errors.Wrap(err, "opening db")
		}
		defer db.Close()

		now := func() time.Time {
			return time.Now()
		}

		s, err = service.NewService(db, logger, now)
	}

	if err != nil {
		return errors.Wrap(err, "creating service")
	}

	ctx := context.Background()

	// Create a dial
	d, err := s.CreateDial(ctx, "dan-middlecote", "ASECRETTOKEN")
	if err != nil {
		return errors.Wrap(err, "creating dial")
	}

	// Update the value of the dial
	err = s.SetDial(ctx, d.ID, "ASECRETTOKEN", 67.0)
	if err != nil {
		return errors.Wrap(err, "setting dial")
	}

	// Retrieve the dial
	d, err = s.GetDial(ctx, d.ID)
	if err != nil {
		return errors.Wrap(err, "getting dial")
	}

	// Print the dial
	jd, err := json.Marshal(d)
	if err != nil {
		return errors.Wrap(err, "marshalling dial")
	}
	fmt.Printf("%v\n", string(jd))

	// create a board
	b, err := s.CreateBoard(ctx, "CDP", "ANOTHERSECRET")
	if err != nil {
		return errors.Wrap(err, "creating board")
	}

	// add a dial to the board
	err = s.SetBoard(ctx, b.ID, "ANOTHERSECRET", []ooohh.DialID{d.ID})
	if err != nil {
		return errors.Wrap(err, "adding dial to board")
	}

	// Update the value of the dial
	err = s.SetDial(ctx, d.ID, "ASECRETTOKEN", 33.0)
	if err != nil {
		return errors.Wrap(err, "setting dial")
	}

	// get the board
	b, err = s.GetBoard(ctx, b.ID)
	if err != nil {
		return errors.Wrap(err, "getting board")
	}

	// print the board
	jb, err := json.Marshal(b)
	if err != nil {
		return errors.Wrap(err, "marshalling board")
	}
	fmt.Printf("%v\n", string(jb))

	return nil
}
