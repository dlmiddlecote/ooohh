package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/dlmiddlecote/ooohh"
	"github.com/dlmiddlecote/ooohh/pkg/mock"
)

// newTestLogger returns a logger usable in tests, and also a struct that captures log lines
// logged via the returned logger. It is possible to change the returned loggers level with the
// available level argument.
func newTestLogger(level zapcore.LevelEnabler) (*zap.SugaredLogger, *observer.ObservedLogs) {
	core, recorded := observer.New(level)
	return zap.New(core).Sugar(), recorded
}

func TestCreateDial(t *testing.T) {

	is := is.New(t)

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service, with CreateDial implemented.
	s := &mock.Service{
		CreateDialFn: func(ctx context.Context, name string, token string) (*ooohh.Dial, error) {
			return &ooohh.Dial{
				ID:        ooohh.DialID("dial"),
				Token:     token,
				Name:      name,
				Value:     0.0,
				UpdatedAt: now,
			}, nil
		},
	}

	// Get an API.
	a := NewAPI(logger, s)

	// Create a new request.
	r, err := http.NewRequest("POST", "/api/dials", strings.NewReader(`{"name": "test", "token": "token"}`))
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the create dial handler.
	a.createDial().ServeHTTP(rr, r)

	// Check that the CreateDial function has been invoked.
	is.True(s.CreateDialInvoked)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusCreated)

	// Check the response body is correct
	var actualBody ooohh.Dial
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.ID, ooohh.DialID("dial"))     // id is the same.
	is.Equal(actualBody.Name, "test")                 // name is the same.
	is.Equal(actualBody.Value, 0.0)                   // value is the same.
	is.Equal(actualBody.UpdatedAt.Unix(), now.Unix()) // updated at time is the same.
	is.Equal(actualBody.Token, "")                    // token is not in response body.

}

func TestCreateDialValidation(t *testing.T) {

	is := is.New(t)

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service, with CreateDial implemented.
	s := &mock.Service{
		CreateDialFn: func(ctx context.Context, name string, token string) (*ooohh.Dial, error) {
			return &ooohh.Dial{
				ID:        ooohh.DialID("dial"),
				Token:     token,
				Name:      name,
				Value:     0.0,
				UpdatedAt: now,
			}, nil
		},
	}

	// Get an API.
	a := NewAPI(logger, s)

	for _, tt := range []struct {
		msg       string
		body      string
		expTitle  string
		expDetail string
	}{{
		msg:       "invalid json body",
		body:      `{"name": "test", "token": "token"`,
		expTitle:  "Validation Error",
		expDetail: "Invalid JSON",
	}, {
		msg:       "missing name",
		body:      `{"token": "token"}`,
		expTitle:  "Validation Error",
		expDetail: "Both `name` and `token` must be provided.",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			// Create a new request.
			r, err := http.NewRequest("POST", "/api/dials", strings.NewReader(tt.body))
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the create dial handler.
			a.createDial().ServeHTTP(rr, r)

			// Check that the CreateDial function has not been invoked.
			is.True(!s.CreateDialInvoked)

			// Check the response status code is correct.
			is.Equal(rr.Code, http.StatusBadRequest)

			// Check the response body is correct
			type body struct {
				Title  string `json:"title"`
				Detail string `json:"detail"`
			}
			var actualBody body
			err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
			is.NoErr(err) // actual body is json.

			is.Equal(actualBody.Title, tt.expTitle)   // title is correct.
			is.Equal(actualBody.Detail, tt.expDetail) // detail is correct.
		})
	}

}

func TestGetDial(t *testing.T) {

}

func TestSetDial(t *testing.T) {

}

func TestCreateBoard(t *testing.T) {

}

func TestGetBoard(t *testing.T) {

}

func TestSetBoard(t *testing.T) {

}
