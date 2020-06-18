package service

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/boltdb/bolt"
	"github.com/matryer/is"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// now is the mocked time for tests
var now = time.Date(2020, time.February, 15, 0, 0, 0, 0, time.UTC)

// newTmpBoltDB return a bolt db instance backed by a new temporary file.
// It returns a function that should be called to cleanup the db.
func newTmpBoltDB(t *testing.T) (*bolt.DB, func() error) {
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

	cleanup := func() error {
		return os.Remove(f.Name())
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

// Ensure dial's level can be updated.

// Ensure dial level cannot be updated if token doesn't match.

// Ensure error is returned if dial doesn't exist.

// Timezone stuff.
