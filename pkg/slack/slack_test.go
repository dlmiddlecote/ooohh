package slack

import (
	"context"
	"errors"
	"fmt"
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

	// Variables that will be updated by the set dial function in the service.
	var setID ooohh.DialID
	var setValue *float64

	// Create mock ooohh.Service.
	ms := &mock.Service{
		CreateDialFn: func(ctx context.Context, name string, token string) (*ooohh.Dial, error) {
			return &ooohh.Dial{
				ID:        ooohh.DialID(fmt.Sprintf("dial-%s", name)),
				Name:      name,
				Token:     token,
				Value:     0.0,
				UpdatedAt: time.Now(),
			}, nil
		},
		SetDialFn: func(ctx context.Context, id ooohh.DialID, token string, value float64) error {

			// Capture set values.
			setID = id
			setValue = &value

			return nil
		},
	}

	// Create service.
	s, err := NewService(logger, db, ms, "salt")
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Set dial for the first time.
	// The dial should be created.
	err = s.SetDialValue(ctx, "team", "user", "name", 66.6)
	is.NoErr(err) // setting dial succeeded.

	// Check that CreateDial was called on the service.
	is.True(ms.CreateDialInvoked) // dial was created.

	// Check that SetDial was called on the service.
	is.True(ms.SetDialInvoked) // dial value was set.

	// Check correct values were set.
	is.True(setID != ooohh.DialID("")) // id is not empty.
	is.True(setValue != nil)           // value was set.
	if setValue != nil {
		is.Equal(*setValue, 66.6) // correct value was set.
	}

	// Capture previous id.
	createdID := setID

	// Reset tracking of function invocations, and capturing vars.
	ms.Reset()
	setID = ooohh.DialID("")
	setValue = nil

	// Set the dial again.
	// The dial should NOT be created.
	err = s.SetDialValue(ctx, "team", "user", "name", 10.0)
	is.NoErr(err) // setting dial succeeded.

	// Check that CreateDial was NOT called on the service.
	is.True(!ms.CreateDialInvoked) // dial was not created.

	// Check that SetDial was called on the service.
	is.True(ms.SetDialInvoked) // dial value was set.

	// Check set values.
	is.True(setID != ooohh.DialID("")) // id is not empty.
	is.Equal(setID, createdID)         // id is the same as before.
	is.True(setValue != nil)           // value was set.
	if setValue != nil {
		is.Equal(*setValue, 10.0) // correct value was set.
	}

	// Reset tracking of function invocations, and capturing vars.
	ms.Reset()
	setID = ooohh.DialID("")
	setValue = nil

	// Set the dial for a different user in the same team.
	// The dial should be created.
	err = s.SetDialValue(ctx, "team", "user2", "name2", 33.3)
	is.NoErr(err) // setting dial succeeded.

	// Check that CreateDial was called on the service.
	is.True(ms.CreateDialInvoked) // dial was created.

	// Check that SetDial was called on the service.
	is.True(ms.SetDialInvoked) // dial value was set.

	// Check that the dial id is different for this new user.
	is.True(setID != ooohh.DialID("")) // id is not empty.
	is.True(setID != createdID)        // new dial id is different for different users.

	// Reset tracking of function invocations, and capturing vars.
	ms.Reset()
	setID = ooohh.DialID("")
	setValue = nil

	// Set the dial for the same user on a different team.
	// The dial should be created.
	err = s.SetDialValue(ctx, "team2", "user", "name3", 50.0)
	is.NoErr(err) // setting dial succeeded.

	// Check that CreateDial was called on the service.
	is.True(ms.CreateDialInvoked) // dial was created.

	// Check that SetDial was called on the service.
	is.True(ms.SetDialInvoked) // dial value was set.

	// Check that the dial id is different for this new user.
	is.True(setID != ooohh.DialID("")) // id is not empty.
	is.True(setID != createdID)        // new dial id is different for different teams.
}

func TestGettingDial(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Variables that will be updated by the set/get dial functions in the service.
	var setID ooohh.DialID
	var setValue *float64
	var getID ooohh.DialID

	// Create mock ooohh.Service.
	ms := &mock.Service{
		CreateDialFn: func(ctx context.Context, name string, token string) (*ooohh.Dial, error) {
			return &ooohh.Dial{
				ID:        ooohh.DialID(fmt.Sprintf("dial-%s", name)),
				Name:      name,
				Token:     token,
				Value:     0.0,
				UpdatedAt: time.Now(),
			}, nil
		},
		SetDialFn: func(ctx context.Context, id ooohh.DialID, token string, value float64) error {
			// Capture values.
			setID = id
			setValue = &value

			return nil
		},
		GetDialFn: func(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error) {
			// Capture id.
			getID = id

			return &ooohh.Dial{
				ID:        id,
				Name:      "name",
				Token:     "token",
				Value:     *setValue,
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	// Create service.
	s, err := NewService(logger, db, ms, "salt")
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Set dial.
	err = s.SetDialValue(ctx, "team", "user", "name", 44.4)
	is.NoErr(err) // setting dial succeeded.

	// Get dial.
	d, err := s.GetDial(ctx, "team", "user")
	is.NoErr(err) // getting dial succeeded.

	// Check underlying service was called.
	is.True(ms.GetDialInvoked)

	// Check dial values are as expected.
	is.Equal(setID, getID)     // dial id used in set is used in get.
	is.Equal(d.ID, getID)      // returned dial id is as expected.
	is.Equal(d.Name, "name")   // returned dial name is as expected (what is returned from GetDial).
	is.Equal(d.Token, "token") // returned dial token is as expected (what is returned from GetDial).
	is.Equal(d.Value, 44.4)    // returned dial value is as expected (what is returned from GetDial).
}

func TestGettingNonExistantDial(t *testing.T) {

	is := is.New(t)

	// Get a Bolt DB.
	db, cleanup := newTmpBoltDB(t)
	defer cleanup()

	// Create logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create mock ooohh.Service.
	ms := &mock.Service{}

	// Create service.
	s, err := NewService(logger, db, ms, "salt")
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Get dial.
	_, err = s.GetDial(ctx, "team", "user")
	is.True(errors.Is(err, ErrDialNotFound)) // dial not found error.

	// Check underlying service was not called.
	is.True(!ms.GetDialInvoked)
}

func TestGettingDialError(t *testing.T) {

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
				ID:        ooohh.DialID(fmt.Sprintf("dial-%s", name)),
				Name:      name,
				Token:     token,
				Value:     0.0,
				UpdatedAt: time.Now(),
			}, nil
		},
		SetDialFn: func(ctx context.Context, id ooohh.DialID, token string, value float64) error {
			return nil
		},
		GetDialFn: func(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error) {
			return nil, errors.New("uh-oh")
		},
	}

	// Create service.
	s, err := NewService(logger, db, ms, "salt")
	is.NoErr(err) // service initializes correctly.

	ctx := context.TODO()

	// Set dial.
	err = s.SetDialValue(ctx, "team", "user", "name", 44.4)
	is.NoErr(err) // setting dial succeeded.

	// Get dial.
	_, err = s.GetDial(ctx, "team", "user")
	is.True(err != nil)                       // error returned.
	is.True(!errors.Is(err, ErrDialNotFound)) // error isn't dial not found error.

	// Check underlying service was called.
	is.True(ms.GetDialInvoked)
}
