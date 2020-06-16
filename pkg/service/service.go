package service

import (
	"context"
	"time"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"github.com/segmentio/ksuid"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/dlmiddlecote/ooohh"
)

var _ ooohh.Service = &service{}

type service struct {
	db  *bolt.DB
	now func() time.Time
}

func NewService(db *bolt.DB, now func() time.Time) (*service, error) {

	// Initialize top-level buckets.
	txn, err := db.Begin(true)
	if err != nil {
		return nil, errors.Wrap(err, "beginning transaction")
	}
	defer txn.Rollback()

	if _, err := txn.CreateBucketIfNotExists([]byte("dials")); err != nil {
		return nil, errors.Wrap(err, "creating dials bucket")
	}

	if _, err := txn.CreateBucketIfNotExists([]byte("boards")); err != nil {
		return nil, errors.Wrap(err, "creating boards bucket")
	}

	return &service{db, now}, txn.Commit()
}

// CreateDial will create the dial with the given name, and associate it to the specified token.
func (s *service) CreateDial(ctx context.Context, name, token string) (*ooohh.Dial, error) {

	// generate new id
	id := ksuid.New().String()

	// start read/write transaction
	txn, err := s.db.Begin(true)
	if err != nil {
		return nil, errors.Wrap(err, "beginning transaction")
	}
	defer txn.Rollback()

	d := ooohh.Dial{
		ID:        id,
		Token:     token,
		Name:      name,
		Value:     0.0,
		UpdatedAt: s.now(),
	}

	if v, err := msgpack.Marshal(d); err != nil {
		return nil, errors.Wrap(err, "marshalling dial")
	} else if err := txn.Bucket([]byte("dials")).Put([]byte(id), v); err != nil {
		return nil, errors.Wrap(err, "storing dial")
	}

	return &d, txn.Commit()
}

// GetDial retrieves a dial by ID. Anyone can retrieve any dial with its ID.
func (s *service) GetDial(ctx context.Context, id string) (*ooohh.Dial, error) {

	// start a read-only transaction
	txn, err := s.db.Begin(false)
	if err != nil {
		return nil, errors.Wrap(err, "beginning transaction")
	}
	defer txn.Rollback()

	var d ooohh.Dial
	if v := txn.Bucket([]byte("dials")).Get([]byte(id)); v == nil {
		return nil, ooohh.ErrDialNotFound
	} else if err := msgpack.Unmarshal(v, &d); err != nil {
		return nil, errors.Wrap(err, "reading dial")
	}
	return &d, nil
}

// SetDial updates the dial value. It can be updated by anyone who knows
// the original token it was created with.
func (s *service) SetDial(ctx context.Context, id, token string, value float64) error {

	// start read/write transaction
	txn, err := s.db.Begin(true)
	if err != nil {
		return errors.Wrap(err, "beginning transaction")
	}
	defer txn.Rollback()

	bkt := txn.Bucket([]byte("dials"))

	// Find and unmarshal dial
	var d ooohh.Dial
	if v := bkt.Get([]byte(id)); v == nil {
		return ooohh.ErrDialNotFound
	} else if err := msgpack.Unmarshal(v, &d); err != nil {
		return errors.Wrap(err, "reading dial")
	}

	// check token matches
	if token != d.Token {
		return ooohh.ErrUnauthorized
	}

	// Update value
	d.Value = value
	d.UpdatedAt = s.now()

	if v, err := msgpack.Marshal(d); err != nil {
		return errors.Wrap(err, "marshalling dial")
	} else if err := bkt.Put([]byte(id), v); err != nil {
		return errors.Wrap(err, "storing dial")
	}

	return txn.Commit()
}

// CreateBoard will create a board with the given name, and associate it to the specified token.
func (s *service) CreateBoard(ctx context.Context, name, token string) (*ooohh.Board, error) {

	// generate new id
	id := ksuid.New().String()

	// start read/write transaction
	txn, err := s.db.Begin(true)
	if err != nil {
		return nil, errors.Wrap(err, "beginning transaction")
	}
	defer txn.Rollback()

	b := ooohh.Board{
		ID:        id,
		Token:     token,
		Name:      name,
		Dials:     []ooohh.Dial{},
		UpdatedAt: s.now(),
	}

	if v, err := msgpack.Marshal(b); err != nil {
		return nil, errors.Wrap(err, "marshalling board")
	} else if err := txn.Bucket([]byte("boards")).Put([]byte(id), v); err != nil {
		return nil, errors.Wrap(err, "storing board")
	}

	return &b, txn.Commit()
}

// GetBoard retrieves a board by ID. Anyone can retrieve any board with its ID.
func (s *service) GetBoard(ctx context.Context, id string) (*ooohh.Board, error) {

	// start a read-only transaction
	txn, err := s.db.Begin(false)
	if err != nil {
		return nil, errors.Wrap(err, "beginning transaction")
	}
	defer txn.Rollback()

	var b ooohh.Board
	if v := txn.Bucket([]byte("boards")).Get([]byte(id)); v == nil {
		return nil, ooohh.ErrBoardNotFound
	} else if err := msgpack.Unmarshal(v, &b); err != nil {
		return nil, errors.Wrap(err, "reading board")
	}
	return &b, nil
}

// SetBoard updates the dials associated with the board. It can be updated
// by anyone who knows the original token it was created with.
func (s *service) SetBoard(ctx context.Context, id, token string, dials []ooohh.Dial) error {

	// start read/write transaction
	txn, err := s.db.Begin(true)
	if err != nil {
		return errors.Wrap(err, "beginning transaction")
	}
	defer txn.Rollback()

	bkt := txn.Bucket([]byte("boards"))

	// Find and unmarshal dial
	var b ooohh.Board
	if v := bkt.Get([]byte(id)); v == nil {
		return ooohh.ErrBoardNotFound
	} else if err := msgpack.Unmarshal(v, &b); err != nil {
		return errors.Wrap(err, "reading board")
	}

	// check token matches
	if token != b.Token {
		return ooohh.ErrUnauthorized
	}

	// Update value
	b.Dials = dials
	b.UpdatedAt = s.now()

	if v, err := msgpack.Marshal(b); err != nil {
		return errors.Wrap(err, "marshalling board")
	} else if err := bkt.Put([]byte(id), v); err != nil {
		return errors.Wrap(err, "storing board")
	}

	return txn.Commit()
}
