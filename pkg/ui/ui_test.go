package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/julienschmidt/httprouter"
	"github.com/matryer/is"

	"github.com/dlmiddlecote/kit/api"
	"github.com/dlmiddlecote/ooohh"
	"github.com/dlmiddlecote/ooohh/pkg/mock"
)

func newRequest(method, path string, body io.Reader, params httprouter.Params) (*http.Request, error) {
	r, err := http.NewRequest(method, path, body)
	if err != nil {
		return r, err
	}

	r = api.SetDetails(r, path, params)

	return r, nil
}

func TestIndexContainsLinkToCreateBoard(t *testing.T) {

	is := is.New(t)

	// Create a mock service.
	s := &mock.Service{}

	// Create the ui struct.
	ui := NewUI(s)

	// Create a new request.
	r, err := http.NewRequest("GET", "/", nil)
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the index handler.
	ui.Index().ServeHTTP(rr, r)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Parse HTML.
	doc, err := goquery.NewDocumentFromReader(rr.Body)
	is.NoErr(err)

	// Check the link is within the html.
	found := false
	doc.Find("a[href]").Each(func(index int, item *goquery.Selection) {
		href, _ := item.Attr("href")
		if href == "/new" {
			found = true
		}
	})

	is.True(found) // link to new board has been found.

}

func TestNewBoardContainsForm(t *testing.T) {

	is := is.New(t)

	// Create a mock service.
	s := &mock.Service{}

	// Create the ui struct.
	ui := NewUI(s)

	// Create a new request.
	r, err := http.NewRequest("GET", "/new", nil)
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the create board handler.
	ui.CreateBoard().ServeHTTP(rr, r)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Parse HTML.
	doc, err := goquery.NewDocumentFromReader(rr.Body)
	is.NoErr(err)

	// Check the form is within the html.
	nameFound := false
	tokenFound := false
	doc.Find(`form[name="create-board"]`).Each(func(index int, item *goquery.Selection) {
		// Find name input within form.
		name := item.Find(`input[name="name"]`)
		if name != nil {
			nameFound = true
		}
		// Find token input within form.
		token := item.Find(`input[name="token"]`)
		if token != nil {
			tokenFound = true
		}
	})

	is.True(nameFound)  // name input element found.
	is.True(tokenFound) // token input element found.

}

func TestCreatingBoardOK(t *testing.T) {

	is := is.New(t)

	// Variables that will be set within the creation of the board.
	var setName string
	var setToken string

	// Create a mock service.
	s := &mock.Service{
		CreateBoardFn: func(ctx context.Context, name string, token string) (*ooohh.Board, error) {
			setName = name
			setToken = token

			return &ooohh.Board{
				ID:        ooohh.BoardID("board-id"),
				Name:      name,
				Token:     token,
				Dials:     []ooohh.Dial{},
				UpdatedAt: time.Now(),
			}, nil
		},
	}

	// Create the ui struct.
	ui := NewUI(s)

	// Create a new request.
	formData := url.Values{
		"name":  {"test-board"},
		"token": {"token"},
	}
	r, err := http.NewRequest("POST", "/new", strings.NewReader(formData.Encode()))
	is.NoErr(err) // request creates ok.
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the create board handler.
	ui.CreateBoard().ServeHTTP(rr, r)

	// Check the board was created.
	is.True(s.CreateBoardInvoked) // board was created.

	// Check the board was created with the correct data.
	is.Equal(setName, "test-board") // name was set correctly.
	is.Equal(setToken, "token")     // token was set correctly.

	// Check the response redirects correctly.
	is.Equal(rr.Code, http.StatusSeeOther)                    // response status code is a redirect.
	is.Equal(rr.Header().Get("Location"), "/boards/board-id") // response location header is to the new board.
}

func TestCreatingBoardValidation(t *testing.T) {

	// Create a mock service.
	s := &mock.Service{}

	// Create the ui struct.
	ui := NewUI(s)

	for _, tt := range []struct {
		msg         string
		form        url.Values
		errMsgs     []string
		missingMsgs []string
	}{{
		msg:         "no name or token",
		form:        url.Values{},
		errMsgs:     []string{"Please enter a name.", "Please enter a token."},
		missingMsgs: []string{},
	}, {
		msg: "no name",
		form: url.Values{
			"token": {"token"},
		},
		errMsgs:     []string{"Please enter a name."},
		missingMsgs: []string{"Please enter a token."},
	}, {
		msg: "no token",
		form: url.Values{
			"name": {"name"},
		},
		errMsgs:     []string{"Please enter a token."},
		missingMsgs: []string{"Please enter a name."},
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a new request.
			r, err := http.NewRequest("POST", "/new", strings.NewReader(tt.form.Encode()))
			is.NoErr(err) // request creates ok.
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the create board handler.
			ui.CreateBoard().ServeHTTP(rr, r)

			// Check the board was not created.
			is.True(!s.CreateBoardInvoked) // board was not created.

			// Check the response status code.
			is.Equal(rr.Code, http.StatusOK) // response status code is correct.

			// Check the validation error is within the html.
			body := rr.Body.String()
			for _, msg := range tt.errMsgs {
				is.True(strings.Contains(body, msg)) // error message is in the html body.
			}

			// Check the missing messages aren't in the html.
			for _, msg := range tt.missingMsgs {
				is.True(!strings.Contains(body, msg)) // error message is not in the html body.
			}
		})
	}
}

func TestCreatingBoardServiceError(t *testing.T) {

	is := is.New(t)

	// Create a mock service.
	s := &mock.Service{
		CreateBoardFn: func(ctx context.Context, name string, token string) (*ooohh.Board, error) {
			return nil, errors.New("uh-oh")
		},
	}

	// Create the ui struct.
	ui := NewUI(s)

	// Create a new request.
	formData := url.Values{
		"name":  {"test-board"},
		"token": {"token"},
	}
	r, err := http.NewRequest("POST", "/new", strings.NewReader(formData.Encode()))
	is.NoErr(err) // request creates ok.
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the create board handler.
	ui.CreateBoard().ServeHTTP(rr, r)

	// Check the board was created.
	is.True(s.CreateBoardInvoked) // board was created.

	// Check the response status code.
	is.Equal(rr.Code, http.StatusOK) // response status code is correct.

	// Check the validation error is within the html.
	body := rr.Body.String()
	is.True(strings.Contains(body, "Error creating board, please try again.")) // error message is in the html body.
}

func TestGetBoardContainsBoardInformation(t *testing.T) {

	is := is.New(t)

	now := time.Now().Truncate(time.Second)

	// Board that will be returned by service.
	board := ooohh.Board{
		ID:    ooohh.BoardID("board-id"),
		Name:  "Testing Board",
		Token: "token",
		Dials: []ooohh.Dial{
			{
				ID:        ooohh.DialID("dial-1"),
				Token:     "token1",
				Name:      "Dial 1",
				Value:     10.0,
				UpdatedAt: now,
			},
			{
				ID:        ooohh.DialID("dial-2"),
				Token:     "token2",
				Name:      "Dial 2",
				Value:     66.6,
				UpdatedAt: now,
			},
		},
		UpdatedAt: now,
	}

	// Create a mock service.
	s := &mock.Service{
		GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
			return &board, nil
		},
	}

	// Create the ui struct.
	ui := NewUI(s)

	// Create a new request.
	r, err := newRequest("GET", "/boards/:id", nil, httprouter.Params{{Key: "id", Value: "board-id"}})
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the get board handler.
	ui.GetBoard().ServeHTTP(rr, r)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	body := rr.Body.String()

	// Check elements exist on the page.
	is.True(strings.Contains(body, board.Name)) // board name is in response.

	for _, dial := range board.Dials {
		is.True(strings.Contains(body, dial.Name))                       // dial name is in response.
		is.True(strings.Contains(body, fmt.Sprintf("%.1f", dial.Value))) // dial value is in response.
	}
}

func TestGetBoardContainsLinksForms(t *testing.T) {

	is := is.New(t)

	now := time.Now().Truncate(time.Second)

	// Board that will be returned by service.
	board := ooohh.Board{
		ID:    ooohh.BoardID("board-id"),
		Name:  "Testing Board",
		Token: "token",
		Dials: []ooohh.Dial{
			{
				ID:        ooohh.DialID("dial-1"),
				Token:     "token1",
				Name:      "Dial 1",
				Value:     10.0,
				UpdatedAt: now,
			},
			{
				ID:        ooohh.DialID("dial-2"),
				Token:     "token2",
				Name:      "Dial 2",
				Value:     66.6,
				UpdatedAt: now,
			},
		},
		UpdatedAt: now,
	}

	// Create a mock service.
	s := &mock.Service{
		GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
			return &board, nil
		},
	}

	// Create the ui struct.
	ui := NewUI(s)

	// Create a new request.
	r, err := newRequest("GET", "/boards/:id", nil, httprouter.Params{{Key: "id", Value: "board-id"}})
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the get board handler.
	ui.GetBoard().ServeHTTP(rr, r)

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Parse HTML.
	doc, err := goquery.NewDocumentFromReader(rr.Body)
	is.NoErr(err)

	// Check the new board link is within the html.
	found := false
	doc.Find("a[href]").Each(func(index int, item *goquery.Selection) {
		href, _ := item.Attr("href")
		if href == "/new" {
			found = true
		}
	})

	is.True(found) // link to new board has been found.

	// Check the form is within the html.
	dialIDFound := false
	tokenFound := false
	doc.Find(`form[name="add-dial"]`).Each(func(index int, item *goquery.Selection) {
		// Find dialID input within form.
		dialID := item.Find(`input[name="dialID"]`)
		if dialID != nil {
			dialIDFound = true
		}
		// Find token input within form.
		token := item.Find(`input[name="token"]`)
		if token != nil {
			tokenFound = true
		}
	})

	is.True(dialIDFound) // dialID input element found.
	is.True(tokenFound)  // token input element found.

}

func TestGettingBoardServiceError(t *testing.T) {

	for _, tt := range []struct {
		msg    string
		err    error
		expMsg string
	}{{
		msg:    "board not found",
		err:    ooohh.ErrBoardNotFound,
		expMsg: "Oops, the board wasn&#39;t found.",
	}, {
		msg:    "unknown error",
		err:    errors.New("oops"),
		expMsg: "Error retrieving board, please try again.",
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a mock service.
			s := &mock.Service{
				GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
					return nil, tt.err
				},
			}

			// Create the ui struct.
			ui := NewUI(s)

			// Create a new request.
			r, err := newRequest("GET", "/boards/:id", nil, httprouter.Params{{Key: "id", Value: "board-id"}})
			is.NoErr(err)

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the get board handler.
			ui.GetBoard().ServeHTTP(rr, r)

			// Check the response status code is correct.
			is.Equal(rr.Code, http.StatusOK)

			// Check the error msg is within the html.
			body := rr.Body.String()
			is.True(strings.Contains(body, tt.expMsg)) // error message is in the html body.
		})
	}
}

func TestAddingDialToBoardOK(t *testing.T) {

	is := is.New(t)

	now := time.Now().Truncate(time.Second)

	// Board that will be returned by service.
	board := ooohh.Board{
		ID:    ooohh.BoardID("board-id"),
		Name:  "Testing Board",
		Token: "token",
		Dials: []ooohh.Dial{
			{
				ID:        ooohh.DialID("dial-1"),
				Token:     "token",
				Name:      "dial-1",
				Value:     10.0,
				UpdatedAt: now,
			},
			{
				ID:        ooohh.DialID("dial-2"),
				Token:     "token",
				Name:      "dial-2",
				Value:     66.6,
				UpdatedAt: now,
			},
		},
		UpdatedAt: now,
	}

	// Variables that will be set within the updating of the board.
	var setID ooohh.BoardID
	var setToken string
	var setDials *[]ooohh.DialID

	// Create a mock service.
	s := &mock.Service{
		GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
			return &board, nil
		},
		SetBoardFn: func(ctx context.Context, id ooohh.BoardID, token string, dials []ooohh.DialID) error {
			// Capture set values.
			setID = id
			setToken = token
			setDials = &dials

			// update board.
			d := make([]ooohh.Dial, len(dials))
			for i := range dials {
				d[i] = ooohh.Dial{
					ID:        dials[i],
					Token:     "token",
					Name:      string(dials[i]),
					Value:     10.0,
					UpdatedAt: now,
				}
			}
			board.Dials = d

			return nil
		},
	}

	// Create the ui struct.
	ui := NewUI(s)

	// Create a new request.
	formData := url.Values{
		"dialID": {"dial-3"},
		"token":  {"token"},
	}
	r, err := newRequest("POST", "/boards/:id", strings.NewReader(formData.Encode()), httprouter.Params{{Key: "id", Value: "board-id"}})
	is.NoErr(err) // request creates ok.
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the get board handler.
	ui.GetBoard().ServeHTTP(rr, r)

	// Check the board was set.
	is.True(s.SetBoardInvoked) // board was updated.

	// Check the board was updated with the correct data.
	is.Equal(setID, ooohh.BoardID("board-id")) // correct board was set.
	is.Equal(setToken, "token")                // token was set correctly.
	is.True(setDials != nil)                   // dials were set.
	if setDials != nil {
		is.Equal(*setDials, []ooohh.DialID{"dial-1", "dial-2", "dial-3"}) // correct dials were set.
	}

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Check the new dial is within the html.
	body := rr.Body.String()
	is.True(strings.Contains(body, "dial-3")) // new dial is in the html body.
}

func TestAddingDialToBoardValidationError(t *testing.T) {

	now := time.Now().Truncate(time.Second)

	// Board that will be returned by service.
	board := ooohh.Board{
		ID:    ooohh.BoardID("board-id"),
		Name:  "Testing Board",
		Token: "token",
		Dials: []ooohh.Dial{
			{
				ID:        ooohh.DialID("dial-1"),
				Token:     "token",
				Name:      "dial-1",
				Value:     10.0,
				UpdatedAt: now,
			},
			{
				ID:        ooohh.DialID("dial-2"),
				Token:     "token",
				Name:      "dial-2",
				Value:     66.6,
				UpdatedAt: now,
			},
		},
		UpdatedAt: now,
	}

	// Create a mock service.
	s := &mock.Service{
		GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
			return &board, nil
		},
	}

	// Create the ui struct.
	ui := NewUI(s)

	for _, tt := range []struct {
		msg         string
		form        url.Values
		errMsgs     []string
		missingMsgs []string
	}{{
		msg:         "no dial id or token",
		form:        url.Values{},
		errMsgs:     []string{"Please enter a dial ID.", "Please enter the board&#39;s token."},
		missingMsgs: []string{},
	}, {
		msg: "no dial id",
		form: url.Values{
			"token": {"token"},
		},
		errMsgs:     []string{"Please enter a dial ID."},
		missingMsgs: []string{"Please enter the board&#39;s token."},
	}, {
		msg: "no token",
		form: url.Values{
			"dialID": {"dial-id"},
		},
		errMsgs:     []string{"Please enter the board&#39;s token."},
		missingMsgs: []string{"Please enter a dial ID."},
	}} {

		t.Run(tt.msg, func(t *testing.T) {

			is := is.New(t)

			// Create a new request.
			r, err := newRequest("POST", "/boards/:id", strings.NewReader(tt.form.Encode()), httprouter.Params{{Key: "id", Value: "board-id"}})
			is.NoErr(err) // request creates ok.
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
			rr := httptest.NewRecorder()

			// Invoke the get board handler.
			ui.GetBoard().ServeHTTP(rr, r)

			// Check the board was not set.
			is.True(!s.SetBoardInvoked) // board was not updated.

			// Check the response status code is correct.
			is.Equal(rr.Code, http.StatusOK)

			// Check the validation error is within the html.
			body := rr.Body.String()
			for _, msg := range tt.errMsgs {
				is.True(strings.Contains(body, msg)) // error message is in the html body.
			}

			// Check the missing messages aren't in the html.
			for _, msg := range tt.missingMsgs {
				is.True(!strings.Contains(body, msg)) // error message is not in the html body.
			}
		})
	}
}

func TestAddingDialToBoardGetBoardError(t *testing.T) {

	is := is.New(t)

	// Create a mock service.
	s := &mock.Service{
		GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
			return nil, errors.New("uh-oh")
		},
	}

	// Create the ui struct.
	ui := NewUI(s)

	// Create a new request.
	formData := url.Values{
		"dialID": {"new-dial-id"},
		"token":  {"entered-token"},
	}
	r, err := newRequest("POST", "/boards/:id", strings.NewReader(formData.Encode()), httprouter.Params{{Key: "id", Value: "board-id"}})
	is.NoErr(err) // request creates ok.
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the get board handler.
	ui.GetBoard().ServeHTTP(rr, r)

	// Check the board was retrieved.
	is.True(s.GetBoardInvoked) // board was retrieved.

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Check the html.
	body := rr.Body.String()
	is.True(strings.Contains(body, "Error retrieving board, please try again.")) // error msg is in the html body.
}

func TestAddingDialToBoardSetBoardError(t *testing.T) {

	is := is.New(t)

	// Create a mock service.
	s := &mock.Service{
		GetBoardFn: func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
			return &ooohh.Board{
				ID:        ooohh.BoardID("board-id"),
				Token:     "token",
				Name:      "Board",
				Dials:     []ooohh.Dial{},
				UpdatedAt: time.Now(),
			}, nil
		},
		SetBoardFn: func(ctx context.Context, id ooohh.BoardID, token string, dials []ooohh.DialID) error {
			return errors.New("uh-oh")
		},
	}

	// Create the ui struct.
	ui := NewUI(s)

	// Create a new request.
	formData := url.Values{
		"dialID": {"new-dial-id"},
		"token":  {"entered-token"},
	}
	r, err := newRequest("POST", "/boards/:id", strings.NewReader(formData.Encode()), httprouter.Params{{Key: "id", Value: "board-id"}})
	is.NoErr(err) // request creates ok.
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the get board handler.
	ui.GetBoard().ServeHTTP(rr, r)

	// Check the board was retrieved.
	is.True(s.GetBoardInvoked) // board was retrieved.

	// Check the board was updated.
	is.True(s.SetBoardInvoked) // board was updated.

	// Check the response status code is correct.
	is.Equal(rr.Code, http.StatusOK)

	// Check the html.
	body := rr.Body.String()
	is.True(strings.Contains(body, "Error adding dial, please try again.")) // error msg is in the html body.
	is.True(strings.Contains(body, "new-dial-id"))                          // entered dial id is still on page.
	is.True(strings.Contains(body, "entered-token"))                        // entered token is still on page.
}
