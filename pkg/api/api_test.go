package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/matryer/is"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/dlmiddlecote/kit/api"
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

func newRequest(method, path string, body io.Reader, params httprouter.Params) (*http.Request, error) {
	r, err := http.NewRequest(method, path, body)
	if err != nil {
		return r, err
	}

	r = api.SetDetails(r, path, params)

	return r, nil
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
	}, {
		msg:       "missing token",
		body:      `{"name": "test"}`,
		expTitle:  "Validation Error",
		expDetail: "Both `name` and `token` must be provided.",
	}, {
		msg:       "missing name & token",
		body:      `{}`,
		expTitle:  "Validation Error",
		expDetail: "Both `name` and `token` must be provided.",
	}, {
		msg:       "extra field passed",
		body:      `{"extra": "field"}`,
		expTitle:  "Validation Error",
		expDetail: "Both `name` and `token` must be provided.",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

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

func TestCreateDialError(t *testing.T) {

	is := is.New(t)

	// Get a logger.
	logger, logs := newTestLogger(zap.InfoLevel)

	// Create a mock service, with CreateDial implemented, that returns an error.
	s := &mock.Service{
		CreateDialFn: func(ctx context.Context, name string, token string) (*ooohh.Dial, error) {
			return nil, errors.New("error message")
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
	is.Equal(rr.Code, http.StatusInternalServerError)

	// Check the response body is correct
	type body struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	var actualBody body
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.Title, "Internal Server Error")  // title is correct.
	is.Equal(actualBody.Detail, "Could not create dial") // detail is correct.

	// Check logs are correct.
	is.Equal(len(logs.FilterMessage("could not create dial").All()), 1)                                          // error is logged.
	is.Equal(logs.FilterMessage("could not create dial").All()[0].ContextMap()["err"].(string), "error message") // error message is logged under error key.
}

func TestGetDial(t *testing.T) {

	is := is.New(t)

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service, with GetDial implemented.
	s := &mock.Service{
		GetDialFn: func(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error) {
			return &ooohh.Dial{
				ID:        id,
				Token:     "token",
				Name:      "test",
				Value:     66.6,
				UpdatedAt: now,
			}, nil
		},
	}

	// Get an API.
	a := NewAPI(logger, s)

	// Create a new request.
	r, err := newRequest("GET", "/api/dials/:id", nil, httprouter.Params{{Key: "id", Value: "1234"}})
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the get dial handler.
	a.getDial().ServeHTTP(rr, r)

	// Check that the GetDial function has been invoked.
	is.True(s.GetDialInvoked)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Check the response body is correct
	var actualBody ooohh.Dial
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.ID, ooohh.DialID("1234"))     // id is correct.
	is.Equal(actualBody.Name, "test")                 // name is correct.
	is.Equal(actualBody.Value, 66.6)                  // value is correct.
	is.Equal(actualBody.UpdatedAt.Unix(), now.Unix()) // updated at time is correct.
	is.Equal(actualBody.Token, "")                    // token is not in response body.
}

func TestGetDialErrors(t *testing.T) {

	is := is.New(t)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	for _, tt := range []struct {
		msg       string
		err       error
		expStatus int
		expTitle  string
		expDetail string
	}{{
		msg:       "dial not found",
		err:       ooohh.ErrDialNotFound,
		expStatus: http.StatusNotFound,
		expTitle:  "Not Found",
		expDetail: "Not Found",
	}, {
		msg:       "unknown error",
		err:       errors.New("uh-oh"),
		expStatus: http.StatusInternalServerError,
		expTitle:  "Internal Server Error",
		expDetail: "Could not retrieve dial",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a mock service, with GetDial implemented.
			s := &mock.Service{
				GetDialFn: func(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error) {
					return nil, tt.err
				},
			}

			// Get an API.
			a := NewAPI(logger, s)

			// Create a new request.
			r, err := newRequest("GET", "/api/dials/:id", nil, httprouter.Params{{Key: "id", Value: "1234"}})
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the get dial handler.
			a.getDial().ServeHTTP(rr, r)

			// Check that the GetDial function has been invoked.
			is.True(s.GetDialInvoked)

			// Check the response status code is correct.
			is.Equal(rr.Code, tt.expStatus)

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

func TestSetDial(t *testing.T) {

}

func TestCreateBoard(t *testing.T) {

}

func TestGetBoard(t *testing.T) {

}

func TestSetBoard(t *testing.T) {

}
