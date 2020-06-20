package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	logger, _ := newTestLogger(zap.InfoLevel)

	s := &mock.Service{
		CreateDialFn: func(ctx context.Context, name string, token string) (*ooohh.Dial, error) {
			return &ooohh.Dial{ID: ooohh.DialID("dial")}, nil
		},
	}

	a := NewAPI(logger, s)

	r, err := http.NewRequest("POST", "/api/dials", strings.NewReader(`{"name": "test", "token": "token"}`))
	is.NoErr(err)

	rr := httptest.NewRecorder()

	a.createDial().ServeHTTP(rr, r)

	is.Equal(rr.Code, http.StatusCreated)

	is.True(s.CreateDialInvoked)
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
