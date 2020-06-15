package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	stdbolt "github.com/boltdb/bolt"
	"github.com/pkg/errors"

	"github.com/dlmiddlecote/ooohh"
	"github.com/dlmiddlecote/ooohh/pkg/bolt"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stdout, "error: %v", err)
		os.Exit(1)
	}
}

func run() error {

	var err error
	var s ooohh.Service
	{
		db, err := stdbolt.Open("/tmp/bolt.db", 0600, nil)
		if err != nil {
			return errors.Wrap(err, "opening db")
		}
		defer db.Close()
	
		now := func() time.Time {
			return time.Now()
		}

		s, err = bolt.NewService(db, now)
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
	err = s.SetBoard(ctx, b.ID, "ANOTHERSECRET", []ooohh.Dial{*d})
	if err != nil {
		return errors.Wrap(err, "adding dial to board")
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