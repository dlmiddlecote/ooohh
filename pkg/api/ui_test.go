package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/matryer/is"

	"github.com/dlmiddlecote/ooohh"
	"github.com/dlmiddlecote/ooohh/pkg/mock"
)

func TestIndexContainsLinkToCreateBoard(t *testing.T) {

	is := is.New(t)

	// Create a mock service.
	s := &mock.Service{}

	// Create the ui struct.
	ui := &ui{s}

	// Create a new request.
	r, err := http.NewRequest("GET", "/", nil)
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the index handler.
	ui.index().ServeHTTP(rr, r)

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
	ui := &ui{s}

	// Create a new request.
	r, err := http.NewRequest("GET", "/new", nil)
	is.NoErr(err)

	// Create a response recorder, which satisfies http.ResponseWriter, to record the response.
	rr := httptest.NewRecorder()

	// Invoke the create board handler.
	ui.createBoard().ServeHTTP(rr, r)

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
	ui := &ui{s}

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
	ui.createBoard().ServeHTTP(rr, r)

	// Check the board was created.
	is.True(s.CreateBoardInvoked) // board was created.

	// Check the board was created with the correct data.
	is.Equal(setName, "test-board") // name was set correctly.
	is.Equal(setToken, "token")     // token was set correctly.

	// Check the response redirects correctly.
	is.Equal(rr.Code, http.StatusTemporaryRedirect)           // response status code is a redirect.
	is.Equal(rr.Header().Get("Location"), "/boards/board-id") // response location header is to the new board.
}
