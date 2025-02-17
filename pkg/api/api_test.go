package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dlmiddlecote/kit/api"
	"github.com/julienschmidt/httprouter"
	"github.com/matryer/is"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/dlmiddlecote/ooohh"
	"github.com/dlmiddlecote/ooohh/pkg/mock"
	"github.com/dlmiddlecote/ooohh/pkg/slack"
	"github.com/dlmiddlecote/ooohh/pkg/ui"
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

func TestOoohhAPIIsKitAPI(t *testing.T) {

	is := is.New(t)

	var i interface{} = &ooohhAPI{}
	_, ok := i.(api.API)
	is.True(ok) // ooohh api is kit api.
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

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

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

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

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

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

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

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

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

			// Create a mock slack service.
			ss := &mock.SlackService{}

			// Create UI.
			ui := ui.NewUI(s)

			// Get an API.
			a := NewAPI(logger, s, ss, ui)

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

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	for _, tt := range []struct {
		msg   string
		value float64
	}{{
		msg:   "non-zero value",
		value: 66.6,
	}, {
		msg:   "zero value",
		value: 0,
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Variables that will be assigned to within the SetDial function.
			var setID ooohh.DialID
			var setToken string
			var setValue *float64

			// Create a mock service, with GetDial and SetDial implemented.
			s := &mock.Service{
				SetDialFn: func(ctx context.Context, id ooohh.DialID, token string, value float64) error {

					// Capture what was set.
					setID = id
					setToken = token
					setValue = &value

					return nil
				},
				GetDialFn: func(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error) {
					return &ooohh.Dial{
						ID:        id,
						Token:     setToken,
						Name:      "test",
						Value:     *setValue,
						UpdatedAt: now,
					}, nil
				},
			}

			// Create a mock slack service.
			ss := &mock.SlackService{}

			// Create UI.
			ui := ui.NewUI(s)

			// Get an API.
			a := NewAPI(logger, s, ss, ui)

			// Create a new request.
			r, err := newRequest("PATCH", "/api/dials/:id", strings.NewReader(fmt.Sprintf(`{"token": "token", "value": %f}`, tt.value)), httprouter.Params{{Key: "id", Value: "1234"}})
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the set dial handler.
			a.setDialValue().ServeHTTP(rr, r)

			// Check that the SetDial function has been invoked.
			is.True(s.SetDialInvoked)

			// Check that the SetDial function was invoked with the correct params.
			is.Equal(setID, ooohh.DialID("1234")) // correct dial was set.
			is.Equal(setToken, "token")           // correct token was used for the set.
			is.True(setValue != nil)              // value was set.
			if setValue != nil {
				is.Equal(*setValue, tt.value) // correct value was set.
			}

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
			is.Equal(actualBody.Value, tt.value)              // value is correct.
			is.Equal(actualBody.UpdatedAt.Unix(), now.Unix()) // updated at time is correct.
			is.Equal(actualBody.Token, "")                    // token is not in response body.
		})
	}

}

func TestSetDialValidation(t *testing.T) {

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service.
	s := &mock.Service{}

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	for _, tt := range []struct {
		msg       string
		body      string
		expTitle  string
		expDetail string
	}{{
		msg:       "invalid json body",
		body:      `{"value": 66.6, "token": "token"`,
		expTitle:  "Validation Error",
		expDetail: "Invalid JSON",
	}, {
		msg:       "missing value",
		body:      `{"token": "token"}`,
		expTitle:  "Validation Error",
		expDetail: "Both `token` and `value` must be provided.",
	}, {
		msg:       "missing token",
		body:      `{"value": 66.6}`,
		expTitle:  "Validation Error",
		expDetail: "Both `token` and `value` must be provided.",
	}, {
		msg:       "missing value & token",
		body:      `{}`,
		expTitle:  "Validation Error",
		expDetail: "Both `token` and `value` must be provided.",
	}, {
		msg:       "extra field passed",
		body:      `{"extra": "field"}`,
		expTitle:  "Validation Error",
		expDetail: "Both `token` and `value` must be provided.",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a new request.
			r, err := newRequest("PATCH", "/api/dials/:id", strings.NewReader(tt.body), httprouter.Params{{Key: "id", Value: "1234"}})
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the set dial handler.
			a.setDialValue().ServeHTTP(rr, r)

			// Check that the SetDial function has not been invoked.
			is.True(!s.SetDialInvoked)

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

func TestSetDialErrors(t *testing.T) {

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	for _, tt := range []struct {
		msg           string
		setErr        error
		getErr        error
		expGetInvoked bool
		expStatus     int
		expTitle      string
		expDetail     string
	}{{
		msg:           "set with wrong token",
		setErr:        ooohh.ErrUnauthorized,
		getErr:        nil,
		expGetInvoked: false,
		expStatus:     http.StatusUnauthorized,
		expTitle:      "Unauthorized",
		expDetail:     "Invalid token",
	}, {
		msg:           "set with missing dial",
		setErr:        ooohh.ErrDialNotFound,
		getErr:        ooohh.ErrDialNotFound,
		expGetInvoked: false,
		expStatus:     http.StatusNotFound,
		expTitle:      "Not Found",
		expDetail:     "Not Found",
	}, {
		msg:           "set with invalid value",
		setErr:        ooohh.ErrDialValueInvalid,
		getErr:        nil,
		expGetInvoked: false,
		expStatus:     http.StatusBadRequest,
		expTitle:      "Bad Request",
		expDetail:     "Invalid value",
	}, {
		msg:           "set with unknown error",
		setErr:        errors.New("set error"),
		getErr:        nil,
		expGetInvoked: false,
		expStatus:     http.StatusInternalServerError,
		expTitle:      "Internal Server Error",
		expDetail:     "Could not update dial",
	}, {
		msg:           "get with unknown error",
		setErr:        nil,
		getErr:        errors.New("get error"),
		expGetInvoked: true,
		expStatus:     http.StatusInternalServerError,
		expTitle:      "Internal Server Error",
		expDetail:     "Could not update dial",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a mock service, with GetDial and SetDial implemented.
			s := &mock.Service{
				SetDialFn: func(ctx context.Context, id ooohh.DialID, token string, value float64) error {
					return tt.setErr
				},
				GetDialFn: func(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error) {
					return &ooohh.Dial{
						ID:        id,
						Token:     "token",
						Name:      "test",
						Value:     66.6,
						UpdatedAt: now,
					}, tt.getErr
				},
			}

			// Create a mock slack service.
			ss := &mock.SlackService{}

			// Create UI.
			ui := ui.NewUI(s)

			// Get an API.
			a := NewAPI(logger, s, ss, ui)

			// Create a new request.
			r, err := newRequest("PATCH", "/api/dials/:id", strings.NewReader(`{"token": "token", "value": 66.6}`), httprouter.Params{{Key: "id", Value: "1234"}})
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the set dial handler.
			a.setDialValue().ServeHTTP(rr, r)

			// Check that the SetDial function has been invoked.
			is.True(s.SetDialInvoked)

			// Check that the GetDial function has (not) been invoked.
			is.Equal(s.GetDialInvoked, tt.expGetInvoked)

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

func TestCreateBoard(t *testing.T) {

	is := is.New(t)

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service, with CreateBoard implemented.
	s := &mock.Service{
		CreateBoardFn: func(ctx context.Context, name string, token string) (*ooohh.Board, error) {
			return &ooohh.Board{
				ID:        ooohh.BoardID("board"),
				Token:     token,
				Name:      name,
				Dials:     []ooohh.Dial{},
				UpdatedAt: now,
			}, nil
		},
	}

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	// Create a new request.
	r, err := http.NewRequest("POST", "/api/boards", strings.NewReader(`{"name": "test", "token": "token"}`))
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the create board handler.
	a.createBoard().ServeHTTP(rr, r)

	// Check that the CreateBoard function has been invoked.
	is.True(s.CreateBoardInvoked)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusCreated)

	// Check the response body is correct
	var actualBody ooohh.Board
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.ID, ooohh.BoardID("board"))   // id is the same.
	is.Equal(actualBody.Name, "test")                 // name is the same.
	is.Equal(actualBody.Dials, []ooohh.Dial{})        // dials are the same.
	is.Equal(actualBody.UpdatedAt.Unix(), now.Unix()) // updated at time is the same.
	is.Equal(actualBody.Token, "")                    // token is not in response body.
}

func TestCreateBoardValidation(t *testing.T) {

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service, with CreateBoard implemented.
	s := &mock.Service{
		CreateBoardFn: func(ctx context.Context, name string, token string) (*ooohh.Board, error) {
			return &ooohh.Board{
				ID:        ooohh.BoardID("board"),
				Token:     token,
				Name:      name,
				Dials:     []ooohh.Dial{},
				UpdatedAt: now,
			}, nil
		},
	}

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

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
			r, err := http.NewRequest("POST", "/api/boards", strings.NewReader(tt.body))
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the create board handler.
			a.createBoard().ServeHTTP(rr, r)

			// Check that the CreateBoard function has not been invoked.
			is.True(!s.CreateBoardInvoked)

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

func TestCreateBoardError(t *testing.T) {

	is := is.New(t)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service, with CreateBoard implemented, that returns an error.
	s := &mock.Service{
		CreateBoardFn: func(ctx context.Context, name string, token string) (*ooohh.Board, error) {
			return nil, errors.New("error message")
		},
	}

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	// Create a new request.
	r, err := http.NewRequest("POST", "/api/boards", strings.NewReader(`{"name": "test", "token": "token"}`))
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the create board handler.
	a.createBoard().ServeHTTP(rr, r)

	// Check that the CreateBoard function has been invoked.
	is.True(s.CreateBoardInvoked)

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

	is.Equal(actualBody.Title, "Internal Server Error")   // title is correct.
	is.Equal(actualBody.Detail, "Could not create board") // detail is correct.
}

func TestGetBoard(t *testing.T) {

	is := is.New(t)

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service, with GetBoard implemented.
	s := &mock.Service{
		GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
			return &ooohh.Board{
				ID:    id,
				Token: "token",
				Name:  "test",
				Dials: []ooohh.Dial{{
					ID:        "dial",
					Token:     "token",
					Name:      "test",
					Value:     66.6,
					UpdatedAt: now,
				}},
				UpdatedAt: now,
			}, nil
		},
	}

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	// Create a new request.
	r, err := newRequest("GET", "/api/boards/:id", nil, httprouter.Params{{Key: "id", Value: "1234"}})
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the get board handler.
	a.getBoard().ServeHTTP(rr, r)

	// Check that the GetBoard function has been invoked.
	is.True(s.GetBoardInvoked)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Check the response body is correct
	var actualBody ooohh.Board
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.ID, ooohh.BoardID("1234"))    // id is correct.
	is.Equal(actualBody.Name, "test")                 // name is correct.
	is.Equal(len(actualBody.Dials), 1)                // dial length is correct.
	is.Equal(actualBody.UpdatedAt.Unix(), now.Unix()) // updated at time is correct.
	is.Equal(actualBody.Token, "")                    // token is not in response body.

	// Check returned dial is correct.
	dial := actualBody.Dials[0]

	is.Equal(dial.ID, ooohh.DialID("dial"))     // dial id is correct.
	is.Equal(dial.Name, "test")                 // dial name is correct.
	is.Equal(dial.Value, 66.6)                  // dial value is correct.
	is.Equal(dial.UpdatedAt.Unix(), now.Unix()) // dial updated at is correct.
	is.Equal(dial.Token, "")                    // dial token is empty.
}

func TestGetBoardErrors(t *testing.T) {

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	for _, tt := range []struct {
		msg       string
		err       error
		expStatus int
		expTitle  string
		expDetail string
	}{{
		msg:       "board not found",
		err:       ooohh.ErrBoardNotFound,
		expStatus: http.StatusNotFound,
		expTitle:  "Not Found",
		expDetail: "Not Found",
	}, {
		msg:       "unknown error",
		err:       errors.New("uh-oh"),
		expStatus: http.StatusInternalServerError,
		expTitle:  "Internal Server Error",
		expDetail: "Could not retrieve board",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a mock service, with GetBoard implemented.
			s := &mock.Service{
				GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
					return nil, tt.err
				},
			}

			// Create a mock slack service.
			ss := &mock.SlackService{}

			// Create UI.
			ui := ui.NewUI(s)

			// Get an API.
			a := NewAPI(logger, s, ss, ui)

			// Create a new request.
			r, err := newRequest("GET", "/api/boards/:id", nil, httprouter.Params{{Key: "id", Value: "1234"}})
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the get board handler.
			a.getBoard().ServeHTTP(rr, r)

			// Check that the GetBoard function has been invoked.
			is.True(s.GetBoardInvoked)

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

func TestSetBoard(t *testing.T) {

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	for _, tt := range []struct {
		msg   string
		dials []string
	}{{
		msg:   "non-empty dials",
		dials: []string{"4321"},
	}, {
		msg:   "empty dials",
		dials: []string{},
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Variables that will be assigned to within the SetBoard function.
			var setID ooohh.BoardID
			var setToken string
			var setDials *[]ooohh.DialID

			// Create a mock service, with GetBoard and SetBoard implemented.
			s := &mock.Service{
				SetBoardFn: func(ctx context.Context, id ooohh.BoardID, token string, dials []ooohh.DialID) error {

					// Capture what was set.
					setID = id
					setToken = token
					setDials = &dials

					return nil
				},
				GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {

					dials := make([]ooohh.Dial, len(*setDials))
					for i := range *setDials {
						dials[i] = ooohh.Dial{
							ID:        (*setDials)[i],
							Name:      fmt.Sprintf("dial-%d", i),
							Token:     "token",
							Value:     66.6,
							UpdatedAt: now,
						}
					}

					return &ooohh.Board{
						ID:        id,
						Token:     setToken,
						Name:      "test",
						Dials:     dials,
						UpdatedAt: now,
					}, nil
				},
			}

			// Create a mock slack service.
			ss := &mock.SlackService{}

			// Create UI.
			ui := ui.NewUI(s)

			// Get an API.
			a := NewAPI(logger, s, ss, ui)

			// Marshal json.
			type request struct {
				Token string   `json:"token"`
				Dials []string `json:"dials"`
			}

			b, err := json.Marshal(request{"token", tt.dials})
			is.NoErr(err) // invalid request json.

			// Create a new request.
			r, err := newRequest("PATCH", "/api/boards/:id", bytes.NewReader(b), httprouter.Params{{Key: "id", Value: "1234"}})
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the set board handler.
			a.setBoardDials().ServeHTTP(rr, r)

			// Check that the SetBoard function has been invoked.
			is.True(s.SetBoardInvoked)

			// Check that the SetBoard function was invoked with the correct params.
			is.Equal(setID, ooohh.BoardID("1234")) // correct board was set.
			is.Equal(setToken, "token")            // correct token was used for the set.
			is.True(setDials != nil)               // dials were set.
			if setDials != nil {
				dialIDs := make([]ooohh.DialID, len(tt.dials))
				for i := range tt.dials {
					dialIDs[i] = ooohh.DialID(tt.dials[i])
				}
				is.Equal(*setDials, dialIDs) // correct dials were set.
			}

			// Check that the GetBoard function has been invoked.
			is.True(s.GetBoardInvoked)

			// Check the response status code is correct.
			is.Equal(rr.Code, http.StatusOK)

			// Check the response body is correct
			var actualBody ooohh.Board
			err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
			is.NoErr(err) // actual body is json.

			is.Equal(actualBody.ID, ooohh.BoardID("1234"))    // id is correct.
			is.Equal(actualBody.Name, "test")                 // name is correct.
			is.Equal(len(actualBody.Dials), len(tt.dials))    // dial length is correct.
			is.Equal(actualBody.UpdatedAt.Unix(), now.Unix()) // updated at time is correct.
			is.Equal(actualBody.Token, "")                    // token is not in response body.
		})
	}
}

func TestSetBoardValidation(t *testing.T) {

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service.
	s := &mock.Service{}

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	for _, tt := range []struct {
		msg       string
		body      string
		expTitle  string
		expDetail string
	}{{
		msg:       "invalid json body",
		body:      `{"dials": ["4321"], "token": "token"`,
		expTitle:  "Validation Error",
		expDetail: "Invalid JSON",
	}, {
		msg:       "missing value",
		body:      `{"token": "token"}`,
		expTitle:  "Validation Error",
		expDetail: "Both `token` and `dials` must be provided.",
	}, {
		msg:       "missing token",
		body:      `{"dials": ["4321"]}`,
		expTitle:  "Validation Error",
		expDetail: "Both `token` and `dials` must be provided.",
	}, {
		msg:       "missing dials & token",
		body:      `{}`,
		expTitle:  "Validation Error",
		expDetail: "Both `token` and `dials` must be provided.",
	}, {
		msg:       "extra field passed",
		body:      `{"extra": "field"}`,
		expTitle:  "Validation Error",
		expDetail: "Both `token` and `dials` must be provided.",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a new request.
			r, err := newRequest("PATCH", "/api/boards/:id", strings.NewReader(tt.body), httprouter.Params{{Key: "id", Value: "1234"}})
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the set board handler.
			a.setBoardDials().ServeHTTP(rr, r)

			// Check that the SetBoard function has not been invoked.
			is.True(!s.SetBoardInvoked)

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

func TestSetBoardErrors(t *testing.T) {

	now := time.Now().Truncate(time.Second)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	for _, tt := range []struct {
		msg           string
		setErr        error
		getErr        error
		expGetInvoked bool
		expStatus     int
		expTitle      string
		expDetail     string
	}{{
		msg:           "set with wrong token",
		setErr:        ooohh.ErrUnauthorized,
		getErr:        nil,
		expGetInvoked: false,
		expStatus:     http.StatusUnauthorized,
		expTitle:      "Unauthorized",
		expDetail:     "Invalid token",
	}, {
		msg:           "set with missing board",
		setErr:        ooohh.ErrBoardNotFound,
		getErr:        ooohh.ErrBoardNotFound,
		expGetInvoked: false,
		expStatus:     http.StatusNotFound,
		expTitle:      "Not Found",
		expDetail:     "Not Found",
	}, {
		msg:           "set with unknown error",
		setErr:        errors.New("set error"),
		getErr:        nil,
		expGetInvoked: false,
		expStatus:     http.StatusInternalServerError,
		expTitle:      "Internal Server Error",
		expDetail:     "Could not update board",
	}, {
		msg:           "get with unknown error",
		setErr:        nil,
		getErr:        errors.New("get error"),
		expGetInvoked: true,
		expStatus:     http.StatusInternalServerError,
		expTitle:      "Internal Server Error",
		expDetail:     "Could not update board",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a mock service, with GetBoard and SetBoard implemented.
			s := &mock.Service{
				SetBoardFn: func(ctx context.Context, id ooohh.BoardID, token string, dials []ooohh.DialID) error {
					return tt.setErr
				},
				GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
					return &ooohh.Board{
						ID:        id,
						Token:     "token",
						Name:      "test",
						Dials:     []ooohh.Dial{},
						UpdatedAt: now,
					}, tt.getErr
				},
			}

			// Create a mock slack service.
			ss := &mock.SlackService{}

			// Create UI.
			ui := ui.NewUI(s)

			// Get an API.
			a := NewAPI(logger, s, ss, ui)

			// Create a new request.
			r, err := newRequest("PATCH", "/api/boards/:id", strings.NewReader(`{"token": "token", "dials": ["4321"]}`), httprouter.Params{{Key: "id", Value: "1234"}})
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the set board handler.
			a.setBoardDials().ServeHTTP(rr, r)

			// Check that the SetBoard function has been invoked.
			is.True(s.SetBoardInvoked)

			// Check that the GetBoard function has (not) been invoked.
			is.Equal(s.GetBoardInvoked, tt.expGetInvoked)

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

func TestSlackCommand(t *testing.T) {

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service.
	s := &mock.Service{}

	for _, tt := range []struct {
		msg               string
		text              string
		expType           string
		expText           string
		expServiceInvoked bool
		expGetInvoked     bool
	}{{
		msg:               "help command",
		text:              "help",
		expType:           "ephemeral",
		expText:           "Use the following format to set a value: `/wtf <number>`",
		expServiceInvoked: false,
	}, {
		msg:               "low level",
		text:              "10",
		expType:           "ephemeral",
		expText:           "Ooohh, I wish I felt like that.",
		expServiceInvoked: true,
	}, {
		msg:               "medium level",
		text:              "55",
		expType:           "ephemeral",
		expText:           "Ooohh, make sure you take a break!",
		expServiceInvoked: true,
	}, {
		msg:               "high level",
		text:              "85",
		expType:           "ephemeral",
		expText:           "Ooohh, make sure you check in with someone, maybe they can help.",
		expServiceInvoked: true,
	}, {
		msg:               "value too high",
		text:              "101",
		expType:           "ephemeral",
		expText:           "Value out of bounds. Please upply number between 0 and 100.",
		expServiceInvoked: true,
	}, {
		msg:               "value too low",
		text:              "-1",
		expType:           "ephemeral",
		expText:           "Value out of bounds. Please upply number between 0 and 100.",
		expServiceInvoked: true,
	}, {
		msg:               "with spaces",
		text:              "           85       ",
		expType:           "ephemeral",
		expText:           "Ooohh, make sure you check in with someone, maybe they can help.",
		expServiceInvoked: true,
	}, {
		msg:               "query command",
		text:              "?",
		expType:           "ephemeral",
		expText:           "Your dial (id) is set to 10.0.",
		expServiceInvoked: false,
		expGetInvoked:     true,
	}, {
		msg:               "empty command",
		text:              "",
		expType:           "ephemeral",
		expText:           "Please supply a single number as your WTF level.",
		expServiceInvoked: false,
	}, {
		msg:               "invalid number command",
		text:              "this isn't a number",
		expType:           "ephemeral",
		expText:           "Please supply a single number as your WTF level.",
		expServiceInvoked: false,
	}, {
		msg:               "nan number command",
		text:              "nan",
		expType:           "ephemeral",
		expText:           "Sneaky. Please supply a _number_ as your WTF level.",
		expServiceInvoked: false,
	}, {
		msg:               "positive infinite number command",
		text:              "+Inf",
		expType:           "ephemeral",
		expText:           "Definitely seek out help! Unfortunately, I only go up to 100.",
		expServiceInvoked: false,
	}, {
		msg:               "negative infinite number command",
		text:              "-Inf",
		expType:           "ephemeral",
		expText:           "Definitely seek out help! Unfortunately, I only go up to 100.",
		expServiceInvoked: false,
	}, {
		msg:               "infinite number command",
		text:              "Inf",
		expType:           "ephemeral",
		expText:           "Definitely seek out help! Unfortunately, I only go up to 100.",
		expServiceInvoked: false,
	}} {

		t.Run(tt.msg, func(t *testing.T) {
			is := is.New(t)

			// Create a mock slack service.
			ss := &mock.SlackService{
				SetDialValueFn: func(ctx context.Context, teamID, userID, userName string, value float64) error {
					if value > 100.0 || value < 0.0 {
						return ooohh.ErrDialValueInvalid
					}
					return nil
				},
				GetDialFn: func(ctx context.Context, teamID, userID string) (*ooohh.Dial, error) {
					return &ooohh.Dial{
						ID:        ooohh.DialID("id"),
						Name:      "dial",
						Token:     "token",
						Value:     10.0,
						UpdatedAt: time.Now(),
					}, nil
				},
			}

			// Create UI.
			ui := ui.NewUI(s)

			// Get an API.
			a := NewAPI(logger, s, ss, ui)

			// Create a new request.
			formData := url.Values{
				"command": {"/wtf"},
				"user_id": {"user"},
				"team_id": {"team"},
				"text":    {tt.text},
			}
			r, err := http.NewRequest("POST", "/api/slack/command", strings.NewReader(formData.Encode()))
			is.NoErr(err)

			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the slack command handler.
			a.slackCommand().ServeHTTP(rr, r)

			// Check the response status code is correct.
			is.Equal(rr.Code, http.StatusOK)

			// Check the slack service was/was not invoked as expected.
			is.Equal(ss.SetDialValueInvoked, tt.expServiceInvoked)

			// Check the GetDial method of the slack service was/was not invoked as expected.
			is.Equal(ss.GetDialInvoked, tt.expGetInvoked)

			// Check the response body is correct.
			type body struct {
				Type string `json:"response_type"`
				Text string `json:"text"`
			}
			var actualBody body
			err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
			is.NoErr(err) // actual body is json.

			is.Equal(actualBody.Type, tt.expType) // type is correct.
			is.Equal(actualBody.Text, tt.expText) // text is correct.
		})
	}
}

func TestSlackCommandServiceError(t *testing.T) {
	is := is.New(t)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service.
	s := &mock.Service{}

	// Create a mock slack service.
	ss := &mock.SlackService{
		SetDialValueFn: func(ctx context.Context, teamID, userID, userName string, value float64) error {
			return errors.New("uh-oh")
		},
	}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	// Create a new request.
	formData := url.Values{
		"command": {"/wtf"},
		"user_id": {"user"},
		"team_id": {"team"},
		"text":    {"55"},
	}
	r, err := http.NewRequest("POST", "/api/slack/command", strings.NewReader(formData.Encode()))
	is.NoErr(err)

	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the slack command handler.
	a.slackCommand().ServeHTTP(rr, r)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Check the slack service was invoked.
	is.True(ss.SetDialValueInvoked)

	// Check the response body is correct.
	type body struct {
		Type string `json:"response_type"`
		Text string `json:"text"`
	}
	var actualBody body
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.Type, "ephemeral")                                                 // type is correct.
	is.Equal(actualBody.Text, "Oops, something didn't quite work out. Please, try again.") // text is correct.
}

func TestSlackCommandGetDialError(t *testing.T) {
	is := is.New(t)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service.
	s := &mock.Service{}

	// Create a mock slack service.
	ss := &mock.SlackService{
		GetDialFn: func(ctx context.Context, teamID, userID string) (*ooohh.Dial, error) {
			return nil, errors.New("uh-oh")
		},
	}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	// Create a new request.
	formData := url.Values{
		"command": {"/wtf"},
		"user_id": {"user"},
		"team_id": {"team"},
		"text":    {"?"},
	}
	r, err := http.NewRequest("POST", "/api/slack/command", strings.NewReader(formData.Encode()))
	is.NoErr(err)

	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the slack command handler.
	a.slackCommand().ServeHTTP(rr, r)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Check the slack get dial was invoked.
	is.True(ss.GetDialInvoked)

	// Check the slack set dial wasn't invoked.
	is.True(!ss.SetDialValueInvoked)

	// Check the response body is correct.
	type body struct {
		Type string `json:"response_type"`
		Text string `json:"text"`
	}
	var actualBody body
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.Type, "ephemeral")                                                 // type is correct.
	is.Equal(actualBody.Text, "Oops, something didn't quite work out. Please, try again.") // text is correct.
}

func TestSlackCommandInvalidCommand(t *testing.T) {
	is := is.New(t)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service.
	s := &mock.Service{}

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	// Create a new request.
	formData := url.Values{
		"command": {"/not-wtf"},
		"user_id": {"user"},
		"team_id": {"team"},
		"text":    {"55"},
	}
	r, err := http.NewRequest("POST", "/api/slack/command", strings.NewReader(formData.Encode()))
	is.NoErr(err)

	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the slack command handler.
	a.slackCommand().ServeHTTP(rr, r)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Check the slack service was not invoked.
	is.True(!ss.SetDialValueInvoked)

	// Check the response body is correct.
	type body struct {
		Type string `json:"response_type"`
		Text string `json:"text"`
	}
	var actualBody body
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.Type, "ephemeral")                             // type is correct.
	is.Equal(actualBody.Text, "Not sure what you mean there, friend.") // text is correct.
}

func TestSlackCommandValidation(t *testing.T) {

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service.
	s := &mock.Service{}

	for _, tt := range []struct {
		msg  string
		data url.Values
	}{{
		msg: "missing command",
		data: url.Values{
			"user_id": {"user"},
			"team_id": {"team"},
			"text":    {"66.6"},
		},
	}, {
		msg: "missing user",
		data: url.Values{
			"command": {"/wtf"},
			"team_id": {"team"},
			"text":    {"66.6"},
		},
	}, {
		msg: "missing team",
		data: url.Values{
			"command": {"/wtf"},
			"user_id": {"user"},
			"text":    {"66.6"},
		},
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a mock slack service.
			ss := &mock.SlackService{}

			// Create UI.
			ui := ui.NewUI(s)

			// Get an API.
			a := NewAPI(logger, s, ss, ui)

			// Create a new request.
			r, err := http.NewRequest("POST", "/api/slack/command", strings.NewReader(tt.data.Encode()))
			is.NoErr(err)

			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the slack command handler.
			a.slackCommand().ServeHTTP(rr, r)

			// Check the response status code is correct.
			is.Equal(rr.Code, http.StatusInternalServerError)

			// Check the slack service was not invoked.
			is.True(!ss.SetDialValueInvoked)

			// Check the response body is correct.
			type body struct {
				Title  string `json:"title"`
				Detail string `json:"detail"`
			}
			var actualBody body
			err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
			is.NoErr(err) // actual body is json.

			is.Equal(actualBody.Title, "Invalid Request")              // title is correct.
			is.Equal(actualBody.Detail, "Could not parse form values") // detail is correct.
		})
	}
}

func TestSlackCommandInvalidForm(t *testing.T) {
	is := is.New(t)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service.
	s := &mock.Service{}

	// Create a mock slack service.
	ss := &mock.SlackService{}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	// Create a new request.
	r, err := http.NewRequest("POST", "/api/slack/command", nil)
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the slack command handler.
	a.slackCommand().ServeHTTP(rr, r)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusInternalServerError)

	// Check the slack service was not invoked.
	is.True(!ss.SetDialValueInvoked)

	// Check the response body is correct.
	type body struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
	var actualBody body
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.Title, "Invalid Request")       // title is correct.
	is.Equal(actualBody.Detail, "Could not parse form") // detail is correct.
}

func TestSlackCommandQueryWithoutPriorSet(t *testing.T) {
	is := is.New(t)

	// Get a logger.
	logger, _ := newTestLogger(zap.InfoLevel)

	// Create a mock service.
	s := &mock.Service{}

	// Create a mock slack service.
	ss := &mock.SlackService{
		GetDialFn: func(ctx context.Context, teamID, userID string) (*ooohh.Dial, error) {
			return nil, slack.ErrDialNotFound
		},
	}

	// Create UI.
	ui := ui.NewUI(s)

	// Get an API.
	a := NewAPI(logger, s, ss, ui)

	// Create a new request.
	formData := url.Values{
		"command": {"/wtf"},
		"user_id": {"user"},
		"team_id": {"team"},
		"text":    {"?"},
	}
	r, err := http.NewRequest("POST", "/api/slack/command", strings.NewReader(formData.Encode()))
	is.NoErr(err)

	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the slack command handler.
	a.slackCommand().ServeHTTP(rr, r)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Check the slack get service was invoked.
	is.True(ss.GetDialInvoked)

	// Check the slack set service was not invoked.
	is.True(!ss.SetDialValueInvoked)

	// Check the response body is correct.
	type body struct {
		Type string `json:"response_type"`
		Text string `json:"text"`
	}
	var actualBody body
	err = json.Unmarshal(rr.Body.Bytes(), &actualBody)
	is.NoErr(err) // actual body is json.

	is.Equal(actualBody.Type, "ephemeral")                                                // type is correct.
	is.Equal(actualBody.Text, "Use the following format to set a value: `/wtf <number>`") // text is correct.
}
