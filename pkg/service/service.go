package service

import (
	"context"
	"time"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
	"github.com/segmentio/ksuid"
	"github.com/vmihailenco/msgpack/v5"
	"go.uber.org/zap"

	"github.com/dlmiddlecote/ooohh"
)

type service struct {
	db     *bolt.DB
	logger *zap.SugaredLogger
	now    func() time.Time
}

func NewService(db *bolt.DB, logger *zap.SugaredLogger, now func() time.Time) (*service, error) {

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

	return &service{db, logger, now}, txn.Commit()
}

// CreateDial will create the dial with the given name, and associate it to the specified token.
func (s *service) CreateDial(ctx context.Context, name, token string) (*ooohh.Dial, error) {

	// generate new id
	id := ooohh.DialID(ksuid.New().String())

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
		UpdatedAt: s.now().UTC(),
	}

	if v, err := msgpack.Marshal(d); err != nil {
		return nil, errors.Wrap(err, "marshalling dial")
	} else if err := txn.Bucket([]byte("dials")).Put([]byte(id), v); err != nil {
		return nil, errors.Wrap(err, "storing dial")
	}

	return &d, txn.Commit()
}

// GetDial retrieves a dial by ID. Anyone can retrieve any dial with its ID.
func (s *service) GetDial(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error) {

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

	// Update timezone.
	d.UpdatedAt = d.UpdatedAt.UTC()

	return &d, nil
}

// SetDial updates the dial value. It can be updated by anyone who knows
// the original token it was created with.
func (s *service) SetDial(ctx context.Context, id ooohh.DialID, token string, value float64) error {

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
	d.UpdatedAt = s.now().UTC()

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
	id := ooohh.BoardID(ksuid.New().String())

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
		UpdatedAt: s.now().UTC(),
	}

	if v, err := msgpack.Marshal(b); err != nil {
		return nil, errors.Wrap(err, "marshalling board")
	} else if err := txn.Bucket([]byte("boards")).Put([]byte(id), v); err != nil {
		return nil, errors.Wrap(err, "storing board")
	}

	return &b, txn.Commit()
}

// GetBoard retrieves a board by ID. Anyone can retrieve any board with its ID.
func (s *service) GetBoard(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {

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

	// Get dial values.
	dials := make([]ooohh.Dial, 0)
	for _, d := range b.Dials {
		dial, err := s.GetDial(ctx, d.ID)
		if err != nil {
			s.logger.Errorw("GetDial error", "id", d.ID, "board", id, "err", err)
			continue
		}
		dials = append(dials, *dial)
	}

	// Associate populated dials to board.
	b.Dials = dials

	// Update timezone.
	b.UpdatedAt = b.UpdatedAt.UTC()

	return &b, nil
}

// SetBoard updates the dials associated with the board. It can be updated
// by anyone who knows the original token it was created with.
func (s *service) SetBoard(ctx context.Context, id ooohh.BoardID, token string, dials []ooohh.DialID) error {

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

	// Check token matches
	if token != b.Token {
		return ooohh.ErrUnauthorized
	}

	// Populate minimal dial.
	// Value not stored on set.
	allDials := make([]ooohh.Dial, len(dials))
	for i := range dials {
		allDials[i] = ooohh.Dial{ID: dials[i]}
	}

	// Update value
	b.Dials = allDials
	b.UpdatedAt = s.now().UTC()

	if v, err := msgpack.Marshal(b); err != nil {
		return errors.Wrap(err, "marshalling board")
	} else if err := bkt.Put([]byte(id), v); err != nil {
		return errors.Wrap(err, "storing board")
	}

	return txn.Commit()
}
