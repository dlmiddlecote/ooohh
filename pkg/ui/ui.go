package ui

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/dlmiddlecote/kit/api"
	"github.com/markbates/pkger"
	"github.com/pkg/errors"

	"github.com/dlmiddlecote/ooohh"
)

type UI struct {
	s ooohh.Service
}

func NewUI(s ooohh.Service) *UI {
	return &UI{s}
}

func (u *UI) Index() http.Handler {
	f, err := pkger.Open("/frontend/templates/index.html")
	tmpl := template.Must(parseFile(f, err))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, nil) //nolint:errcheck
	})
}

type boardInfo struct {
	Name   string
	Token  string
	Errors map[string]string
}

func (b *boardInfo) Validate() bool {
	b.Errors = make(map[string]string)

	if strings.TrimSpace(b.Name) == "" {
		b.Errors["Name"] = "Please enter a name."
	}

	if strings.TrimSpace(b.Token) == "" {
		b.Errors["Token"] = "Please enter a token."
	}

	return len(b.Errors) == 0
}

func (u *UI) CreateBoard() http.Handler {
	f, err := pkger.Open("/frontend/templates/newboard.html")
	tmpl := template.Must(parseFile(f, err))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			tmpl.Execute(w, nil) //nolint:errcheck
			return
		}

		body := &boardInfo{
			Name:  r.PostFormValue("name"),
			Token: r.PostFormValue("token"),
		}

		if !body.Validate() {
			tmpl.Execute(w, body) //nolint:errcheck
			return
		}

		board, err := u.s.CreateBoard(r.Context(), body.Name, body.Token)
		if err != nil {
			// add a dummy error to the body to return.
			body.Errors["CreateBoard"] = "Error creating board, please try again."

			tmpl.Execute(w, body) //nolint:errcheck
			return
		}

		api.Redirect(w, r, fmt.Sprintf("/boards/%s", board.ID), http.StatusSeeOther)
	})
}

type boardDialInfo struct {
	DialID     string
	BoardToken string
	Errors     map[string]string
}

func (b *boardDialInfo) Validate() bool {
	b.Errors = make(map[string]string)

	if strings.TrimSpace(b.DialID) == "" {
		b.Errors["DialID"] = "Please enter a dial ID."
	}

	if strings.TrimSpace(b.BoardToken) == "" {
		b.Errors["BoardToken"] = "Please enter the board's token."
	}

	return len(b.Errors) == 0
}

func (u *UI) GetBoard() http.Handler {
	f, err := pkger.Open("/frontend/templates/board.html")
	tmpl := template.Must(parseFile(f, err))

	f, err = pkger.Open("/frontend/templates/error.html")
	errTmpl := template.Must(parseFile(f, err))

	type response struct {
		Board         ooohh.Board
		BoardDialInfo *boardDialInfo
	}

	type errResp struct {
		Msg string
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ooohh.BoardID(api.URLParam(r, "id"))

		// Retrieve the board.
		board, err := u.s.GetBoard(r.Context(), id)
		if err != nil {
			msg := "Error retrieving board, please try again."
			if errors.Is(err, ooohh.ErrBoardNotFound) {
				msg = "Oops, the board wasn't found."
			}
			errTmpl.Execute(w, errResp{Msg: msg}) //nolint:errcheck
			return
		}

		if r.Method == "GET" {
			// Display the board.
			tmpl.Execute(w, response{*board, nil}) //nolint:errcheck
			return
		}

		body := boardDialInfo{
			DialID:     r.PostFormValue("dialID"),
			BoardToken: r.PostFormValue("token"),
		}

		if !body.Validate() {
			tmpl.Execute(w, response{*board, &body}) //nolint:errcheck
			return
		}

		dials := make([]ooohh.DialID, len(board.Dials)+1)
		for i := range board.Dials {
			dials[i] = board.Dials[i].ID
		}

		dials[len(board.Dials)] = ooohh.DialID(body.DialID)

		err = u.s.SetBoard(r.Context(), id, body.BoardToken, dials)
		if err != nil {
			// add a dummy error to the body to return.
			body.Errors["SetBoard"] = "Error adding dial, please try again."

			tmpl.Execute(w, response{*board, &body}) //nolint:errcheck
			return
		}

		board, err = u.s.GetBoard(r.Context(), id)
		if err != nil {
			errTmpl.Execute(w, errResp{Msg: "Error retrieving board, please try again."}) //nolint:errcheck
			return
		}

		tmpl.Execute(w, response{*board, nil}) //nolint:errcheck

	})
}

func parseFile(f io.Reader, err error) (*template.Template, error) {
	if err != nil {
		return nil, errors.Wrap(err, "opening file")
	}

	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.Wrap(err, "reading file contents")
	}

	return template.New("").Parse(string(b))
}
