package slack

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/dlmiddlecote/ooohh"
)

// Service represents a service for managing dials from slack commands.
type Service interface {
	// SetDialValue updates the given user's dial value.
	SetDialValue(ctx context.Context, teamID, userID string, value float64) error
}

type service struct {
	s      ooohh.Service
	db     *bolt.DB
	logger *zap.SugaredLogger
}

func NewService(logger *zap.SugaredLogger, db *bolt.DB, s ooohh.Service) (*service, error) {

	// Initialize top-level buckets.
	txn, err := db.Begin(true)
	if err != nil {
		return nil, errors.Wrap(err, "beginning transaction")
	}
	defer txn.Rollback()

	if _, err := txn.CreateBucketIfNotExists([]byte("slack_users")); err != nil {
		return nil, errors.Wrap(err, "creating slack_users bucket")
	}

	return &service{s, db, logger}, txn.Commit()
}

// SetDialValue updates the given user's dial value.
func (s *service) SetDialValue(ctx context.Context, teamID, userID string, value float64) error {

	key := fmt.Sprintf("%s||%s", teamID, userID)
	token := generateToken(key)

	// start a read-only transaction
	rtxn, err := s.db.Begin(false)
	if err != nil {
		return errors.Wrap(err, "starting read transaction")
	}
	defer rtxn.Rollback()

	// Try to retrieve the dial identifier for this user.
	var dialID ooohh.DialID
	if v := rtxn.Bucket([]byte("slack_users")).Get([]byte(key)); v != nil {
		// Dial ID found, convert.
		dialID = ooohh.DialID(v)
	} else {
		// User doesn't exist, create a dial.
		dial, err := s.s.CreateDial(ctx, key, token)
		if err != nil {
			return errors.Wrap(err, "creating board")
		}

		dialID = dial.ID

		// start read/write transaction
		rwtxn, err := s.db.Begin(true)
		if err != nil {
			return errors.Wrap(err, "starting rw transaction")
		}
		defer rwtxn.Rollback()

		// Store user -> dial mapping.
		err = rwtxn.Bucket([]byte("slack_users")).Put([]byte(key), []byte(dialID))
		if err != nil {
			return errors.Wrap(err, "storing user to dial mapping")
		}

		err = rwtxn.Commit()
		if err != nil {
			return errors.Wrap(err, "committing user to dial mapping")
		}
	}

	// Update dial value.
	err = s.s.SetDial(ctx, dialID, token, value)
	if err != nil {
		return errors.Wrap(err, "setting dial value")
	}

	return nil
}

func generateToken(key string) string {
	// base64 encode key
	e := base64.StdEncoding.EncodeToString([]byte(key))

	// lower case the encoded string
	return strings.ToLower(e)
}
