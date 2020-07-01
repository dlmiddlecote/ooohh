package slack

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

	"github.com/dlmiddlecote/ooohh"
	"github.com/dlmiddlecote/ooohh/pkg/mock"
)

// newTmpBoltDB return a bolt db instance backed by a new temporary file.
// It returns a function that should be called to cleanup the db.
func newTmpBoltDB(t *testing.T) (*bolt.DB, func()) {
	// Get temporary filename.
	f, err := ioutil.TempFile("", "ooohh-bolt-slack-")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Create bolt db.
	db, err := bolt.Open(f.Name(), 0600, &bolt.Options{Timeout: 5 * time.Second})
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

func TestServiceIsSlackService(t *testing.T) {

	is := is.New(t)

	var i interface{} = &service{}
	_, ok := i.(Service)
	is.True(ok) // service is a slack service.
}

func TestGenerateTokenAbstractsKey(t *testing.T) {

	for _, tt := range []struct {
		msg string
		key string
	}{{
		msg: "non-empty key",
		key: "thisisakey",
	}, {
		msg: "empty key",
		key: "",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			token := generateToken(tt.key, "salt")

			is.True(token != tt.key) // token is not the same as key
		})
	}
}

func TestSettingDial(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create mock ooohh.Service.
	ms := &mock.Service{
		CreateDialFn: func(ctx context.Context, name string, token string) (*ooohh.Dial, error) {
			return &ooohh.Dial{
				ID:        ooohh.DialID("dial"),
				Name:      name,
				Token:     token,
				Value:     0.0,
				UpdatedAt: time.Now(),
			}, nil
		},
		SetDialFn: func(ctx context.Context, id ooohh.DialID, token string, value float64) error {
			return nil
		},
	}

	// Create service.
	s, err := NewService(logger, db, ms, "salt")
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Set dial for the first time.
	err = s.SetDialValue(ctx, "team", "user", 66.6)
	is.NoErr(err) // setting dial succeeded.

	// Check that CreateDial was called on the service.
	is.True(ms.CreateDialInvoked) // dial was created.

	// Check that SetDial was called on the service.
	is.True(ms.SetDialInvoked) // dial value was set.

	// TODO: Check values that were set etc.

	// Reset tracking of function invocations.
	ms.Reset()

	// Set the dial again.
	err = s.SetDialValue(ctx, "team", "user", 10.0)
	is.NoErr(err) // setting dial succeeded.

	// Check that CreateDial was NOT called on the service.
	is.True(!ms.CreateDialInvoked) // dial was not created.

	// Check that SetDial was called on the service.
	is.True(ms.SetDialInvoked) // dial value was set.
}
