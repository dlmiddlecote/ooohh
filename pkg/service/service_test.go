package service

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	"github.com/dlmiddlecote/ooohh"
	"github.com/matryer/is"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// now is the mocked time for tests
var now = time.Date(2020, time.February, 15, 0, 0, 0, 0, time.UTC)

// newTmpBoltDB return a bolt db instance backed by a new temporary file.
// It returns a function that should be called to cleanup the db.
func newTmpBoltDB(t *testing.T) (*bolt.DB, func()) {
	// Get temporary filename.
	f, err := ioutil.TempFile("", "ooohh-bolt-")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Create bolt db.
	db, err := bolt.Open(f.Name(), 0600, nil)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		os.Remove(f.Name()) //nolint:errcheck
	}

	return db, cleanup
}

// newTestLogger returns a logger usable in tests, and also a struct that captures log lines
// logged via the returned logger. It is possible to change the returned loggers level with the
// available level argument.
func newTestLogger(level zapcore.LevelEnabler) (*zap.SugaredLogger, *observer.ObservedLogs) {
	core, recorded := observer.New(level)
	return zap.New(core).Sugar(), recorded
}

func TestBoltServiceIsOoohhService(t *testing.T) {

	is := is.New(t)

	var i interface{} = &service{}
	_, ok := i.(ooohh.Service)
	is.True(ok) // bolt service is ooohh service.
}

func TestDialCanBeCreatedAndGot(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create service.
	n := func() time.Time {
		return now
	}
	s, err := NewService(db, logger, n)
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Create dial.
	dp, err := s.CreateDial(ctx, "TEST-DIAL-1", "MYTOKEN")
	is.NoErr(err) // dial creates correctly.

	d := *dp

	is.Equal(d.Name, "TEST-DIAL-1") // dial name is correct.
	is.Equal(d.Token, "MYTOKEN")    // dial token is correct.
	is.Equal(d.Value, float64(0))   // dial value is correct.
	is.Equal(d.UpdatedAt, now)      // dial updated at is set.
	is.True(string(d.ID) != "")     // dial id is not empty.

	// Get dial.
	dp, err = s.GetDial(ctx, d.ID)
	is.NoErr(err) // dial is retrieved correctly.

	d2 := *dp

	is.Equal(d2.Name, "TEST-DIAL-1") // dial name is correct.
	is.Equal(d2.Token, "MYTOKEN")    // dial token is correct.
	is.Equal(d2.Value, float64(0))   // dial value is correct.
	is.Equal(d2.UpdatedAt, now)      // dial updated at is correct.
	is.Equal(d2.ID, d.ID)            // dial id is correct.
}

func TestDialValueUpdates(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create service.
	n := func() time.Time {
		return now
	}
	s, err := NewService(db, logger, n)
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Create dial.
	dp, err := s.CreateDial(ctx, "TEST-DIAL-2", "MYTOKEN")
	is.NoErr(err) // dial creates correctly.

	d := *dp

	// Update Dial Value.
	err = s.SetDial(ctx, d.ID, "MYTOKEN", 64.0)
	is.NoErr(err) // dial value sets without error.

	// Check Dial Value.
	dp, err = s.GetDial(ctx, d.ID)
	is.NoErr(err)                     // dial is retrieved correctly.
	is.Equal(dp.Value, float64(64.0)) // dial has correct value.
}

func TestDialValueSetUnauthorized(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create service.
	n := func() time.Time {
		return now
	}
	s, err := NewService(db, logger, n)
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Create dial.
	dp, err := s.CreateDial(ctx, "TEST-DIAL-3", "MYTOKEN")
	is.NoErr(err) // dial creates correctly.

	d := *dp

	// Try to update Dial Value.
	err = s.SetDial(ctx, d.ID, "NOTMYTOKEN", 64.0)
	is.Equal(err, ooohh.ErrUnauthorized) // dial value setting errors as unauthorized.

}

func TestDialNotFound(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create service.
	n := func() time.Time {
		return now
	}
	s, err := NewService(db, logger, n)
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Getting non-existant dial errors.
	_, err = s.GetDial(ctx, ooohh.DialID("NOT-A-DIAL"))
	is.Equal(err, ooohh.ErrDialNotFound) // Dial not found when getting.

	// Updating a non-existant dial errors.
	serr := s.SetDial(ctx, ooohh.DialID("NOT-A-DIAL-EITHER"), "MYTOKEN", 44.0)
	is.Equal(serr, ooohh.ErrDialNotFound) // Dial not found when setting.
}

// Timezone stuff.
func TestStoringTimezones(t *testing.T) {
	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create service.
	n := func() time.Time {
		// return time in new timezone
		return now.In(time.FixedZone("My/Zone", 60*60))
	}
	s, err := NewService(db, logger, n)
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Create dial.
	dp, err := s.CreateDial(ctx, "TEST-DIAL-4", "MYTOKEN")
	is.NoErr(err) // dial creates correctly.

	// Get dial.
	dp, err = s.GetDial(ctx, dp.ID)
	is.NoErr(err) // dial is retrieved correctly.

	// Check time is returned in utc.
	is.Equal(dp.UpdatedAt, now) // time location is UTC.
}

func TestBoardCanBeCreatedAndGot(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create service.
	n := func() time.Time {
		return now
	}
	s, err := NewService(db, logger, n)
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Create board.
	bp, err := s.CreateBoard(ctx, "TEST-BOARD-1", "MYTOKEN")
	is.NoErr(err) // board creates correctly.

	is.Equal(bp.Name, "TEST-BOARD-1")  // board name is correct.
	is.Equal(bp.Token, "MYTOKEN")      // board token is correct.
	is.Equal(bp.Dials, []ooohh.Dial{}) // board dials are empty.
	is.Equal(bp.UpdatedAt, now)        // board updated at is set.
	is.True(string(bp.ID) != "")       // board id is not empty.

	// Get board.
	b2, err := s.GetBoard(ctx, bp.ID)
	is.NoErr(err) // board is retrieved correctly.

	is.Equal(b2.Name, "TEST-BOARD-1")  // board name is correct.
	is.Equal(b2.Token, "MYTOKEN")      // board token is correct.
	is.Equal(b2.Dials, []ooohh.Dial{}) // board dials are empty.
	is.Equal(b2.UpdatedAt, now)        // board updated at is correct.
	is.Equal(b2.ID, bp.ID)             // board id is correct.
}

func TestBoardDialUpdates(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, logs := newTestLogger(zap.InfoLevel)

	// Create service.
	n := func() time.Time {
		return now
	}
	s, err := NewService(db, logger, n)
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Create dial.
	dp, err := s.CreateDial(ctx, "TEST-DIAL", "MYTOKEN")
	is.NoErr(err) // dial creates correctly.

	// Create board.
	bp, err := s.CreateBoard(ctx, "TEST-BOARD-2", "MYTOKEN")
	is.NoErr(err) // board creates correctly.

	// Add dial to board.
	err = s.SetBoard(ctx, bp.ID, "MYTOKEN", []ooohh.DialID{dp.ID})
	is.NoErr(err) // dial added to board without error.

	// Get board.
	bp, err = s.GetBoard(ctx, bp.ID)
	is.NoErr(err)                           // board is retrieved correctly.
	is.Equal(len(bp.Dials), 1)              // board has 1 dial.
	is.Equal(bp.Dials[0].Value, float64(0)) // board dial has 0 value.

	// Update Dial Value.
	err = s.SetDial(ctx, dp.ID, "MYTOKEN", 64.0)
	is.NoErr(err) // dial value sets without error.

	// Get board.
	bp, err = s.GetBoard(ctx, bp.ID)
	is.NoErr(err)                              // board is retrieved correctly.
	is.Equal(len(bp.Dials), 1)                 // board has 1 dial.
	is.Equal(bp.Dials[0].Value, float64(64.0)) // board dial has correct value.

	// Add non-existant dial to board.
	err = s.SetBoard(ctx, bp.ID, "MYTOKEN", []ooohh.DialID{dp.ID, ooohh.DialID("NON-EXISTANT")})
	is.NoErr(err) // dial added to board without error.

	// Get board.
	bp, err = s.GetBoard(ctx, bp.ID)
	is.NoErr(err)                              // board is retrieved correctly.
	is.Equal(len(bp.Dials), 1)                 // board only has 1 dial.
	is.Equal(bp.Dials[0].Value, float64(64.0)) // board dial has correct value.

	// Check non-existant board logs.
	is.Equal(len(logs.FilterMessage("GetDial error").All()), 1) // error is logged.
}

func TestBoardDialSetUnauthorized(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create service.
	n := func() time.Time {
		return now
	}
	s, err := NewService(db, logger, n)
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Create board.
	bp, err := s.CreateBoard(ctx, "TEST-BOARD-3", "MYTOKEN")
	is.NoErr(err) // board creates correctly.

	// Try to update Board Value.
	err = s.SetBoard(ctx, bp.ID, "NOTMYTOKEN", []ooohh.DialID{ooohh.DialID("DIAL")})
	is.Equal(err, ooohh.ErrUnauthorized) // board dials setting errors as unauthorized.

}

func TestBoardNotFound(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create service.
	n := func() time.Time {
		return now
	}
	s, err := NewService(db, logger, n)
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Getting non-existant board errors.
	_, err = s.GetBoard(ctx, ooohh.BoardID("NOT-A-BOARD"))
	is.Equal(err, ooohh.ErrBoardNotFound) // Board not found when getting.

	// Updating a non-existant board errors.
	serr := s.SetBoard(ctx, ooohh.BoardID("NOT-A-DIAL-EITHER"), "MYTOKEN", []ooohh.DialID{})
	is.Equal(serr, ooohh.ErrBoardNotFound) // Board not found when setting.
}
