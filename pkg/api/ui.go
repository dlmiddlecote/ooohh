package api

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

type ui struct {
	s ooohh.Service
}

func (u *ui) index() http.Handler {
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

func (u *ui) createBoard() http.Handler {
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
			tmpl.Execute(w, nil) //nolint:errcheck
			return
		}

		api.Redirect(w, r, fmt.Sprintf("/boards/%s", board.ID), http.StatusTemporaryRedirect)
	})
}

func (u *ui) getBoard() http.Handler {
	f, err := pkger.Open("/frontend/templates/board.html")
	tmpl := template.Must(parseFile(f, err))

	type request struct {
		DialID     string
		BoardToken string
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ooohh.BoardID(api.URLParam(r, "id"))

		if r.Method != "POST" {
			board, err := u.s.GetBoard(r.Context(), id)
			if err != nil {
				api.Redirect(w, r, "/", http.StatusTemporaryRedirect)
				return
			}

			tmpl.Execute(w, *board) //nolint:errcheck
			return
		}

		board, err := u.s.GetBoard(r.Context(), id)
		if err != nil {
			api.Redirect(w, r, fmt.Sprintf("/boards/%s", id), http.StatusTemporaryRedirect)
			return
		}

		err = r.ParseForm()
		if err != nil {
			api.Redirect(w, r, fmt.Sprintf("/boards/%s", id), http.StatusTemporaryRedirect)
			return
		}

		body := request{
			DialID:     r.FormValue("dialID"),
			BoardToken: r.FormValue("token"),
		}

		if body.DialID == "" || body.BoardToken == "" {
			api.Redirect(w, r, fmt.Sprintf("/boards/%s", id), http.StatusTemporaryRedirect)
			return
		}

		dials := make([]ooohh.DialID, len(board.Dials)+1)
		for i := range board.Dials {
			dials[i] = board.Dials[i].ID
		}

		dials[len(board.Dials)] = ooohh.DialID(body.DialID)

		_ = u.s.SetBoard(r.Context(), id, body.BoardToken, dials)

		board, err = u.s.GetBoard(r.Context(), id)
		if err != nil {
			api.Redirect(w, r, fmt.Sprintf("/boards/%s", id), http.StatusTemporaryRedirect)
			return
		}

		tmpl.Execute(w, *board) //nolint:errcheck

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
