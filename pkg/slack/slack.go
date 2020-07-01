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

	salt string
}

func NewService(logger *zap.SugaredLogger, db *bolt.DB, s ooohh.Service, salt string) (*service, error) {

	// Initialize top-level buckets.
	txn, err := db.Begin(true)
	if err != nil {
		return nil, errors.Wrap(err, "beginning transaction")
	}
	defer txn.Rollback() //nolint:errcheck

	if _, err := txn.CreateBucketIfNotExists([]byte("slack_users")); err != nil {
		return nil, errors.Wrap(err, "creating slack_users bucket")
	}

	return &service{s, db, logger, salt}, txn.Commit()
}

// SetDialValue updates the given user's dial value.
func (s *service) SetDialValue(ctx context.Context, teamID, userID string, value float64) error {

	key := fmt.Sprintf("%s:%s", teamID, userID)
	token := generateToken(key, s.salt)

	// Try to retrieve the dial identifier for this user.
	var dialID *ooohh.DialID
	err := s.db.View(func(txn *bolt.Tx) error {
		if v := txn.Bucket([]byte("slack_users")).Get([]byte(key)); v != nil {
			d := ooohh.DialID(v)
			dialID = &d
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "finding existing dial")
	}

	// If the dialID wasn't set before, create a new dial.
	if dialID == nil {
		dial, err := s.s.CreateDial(ctx, key, token)
		if err != nil {
			return errors.Wrap(err, "creating board")
		}

		// Store user -> dial mapping.
		err = s.db.Update(func(txn *bolt.Tx) error {
			err := txn.Bucket([]byte("slack_users")).Put([]byte(key), []byte(dial.ID))
			if err != nil {
				return errors.Wrap(err, "storing user to dial mapping")
			}

			return nil
		})
		if err != nil {
			return errors.Wrap(err, "storing dial mapping")
		}

		// Capture dial ID
		dialID = &dial.ID
	}

	// Update dial value.
	err = s.s.SetDial(ctx, *dialID, token, value)
	if err != nil {
		return errors.Wrap(err, "setting dial value")
	}

	return nil
}

func generateToken(key, salt string) string {
	// Append salt
	key = fmt.Sprintf("%s:%s", key, salt)

	// base64 encode key
	e := base64.StdEncoding.EncodeToString([]byte(key))

	// lower case the encoded string
	return strings.ToLower(e)
}
