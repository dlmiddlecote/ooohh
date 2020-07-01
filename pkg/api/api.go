package api

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/dlmiddlecote/kit/api"
	"github.com/dlmiddlecote/ooohh/pkg/slack"
	"go.uber.org/zap"

	"github.com/dlmiddlecote/ooohh"
)

type ooohhAPI struct {
	logger *zap.SugaredLogger
	s      ooohh.Service
	ss     slack.Service
}

// NewAPI returns an implementation of api.API.
// The returned API exposes the given ooohh service as an HTTP API.
// The Slack command webhook is also exposed.
func NewAPI(logger *zap.SugaredLogger, s ooohh.Service, ss slack.Service) *ooohhAPI {
	return &ooohhAPI{logger, s, ss}
}

// Endpoints implements api.API. We list all API endpoints here.
func (a *ooohhAPI) Endpoints() []api.Endpoint {
	return []api.Endpoint{
		{
			Method:  "POST",
			Path:    "/api/dials",
			Handler: a.createDial(),
		},
		{
			Method:  "GET",
			Path:    "/api/dials/:id",
			Handler: a.getDial(),
		},
		{
			Method:  "PATCH",
			Path:    "/api/dials/:id",
			Handler: a.setDialValue(),
		},
		{
			Method:  "POST",
			Path:    "/api/boards",
			Handler: a.createBoard(),
		},
		{
			Method:  "GET",
			Path:    "/api/boards/:id",
			Handler: a.getBoard(),
		},
		{
			Method:  "PATCH",
			Path:    "/api/boards/:id",
			Handler: a.setBoardDials(),
		},
		{
			Method:  "POST",
			Path:    "/api/slack/command",
			Handler: a.slackCommand(),
		},
	}
}

func (a *ooohhAPI) createDial() http.Handler {
	type request struct {
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	type response ooohh.Dial

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body request
		err := api.Decode(w, r, &body)
		if err != nil {
			api.Problem(w, r, "Validation Error", "Invalid JSON", http.StatusBadRequest)
			return
		}

		if body.Name == "" || body.Token == "" {
			api.Problem(w, r, "Validation Error", "Both `name` and `token` must be provided.", http.StatusBadRequest)
			return
		}

		d, err := a.s.CreateDial(r.Context(), body.Name, body.Token)
		if err != nil {
			a.logger.Errorw("could not create dial", "err", err)
			api.Problem(w, r, "Internal Server Error", "Could not create dial", http.StatusInternalServerError)
			return
		}

		api.Respond(w, r, http.StatusCreated, response(*d))
	})
}

func (a *ooohhAPI) getDial() http.Handler {
	type response ooohh.Dial

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ooohh.DialID(api.URLParam(r, "id"))

		d, err := a.s.GetDial(r.Context(), id)
		if err != nil {
			if errors.Is(err, ooohh.ErrDialNotFound) {
				api.NotFound(w, r)
				return
			}

			a.logger.Errorw("could not retrieve dial", "err", err, "id", id)
			api.Problem(w, r, "Internal Server Error", "Could not retrieve dial", http.StatusInternalServerError)
			return
		}

		api.Respond(w, r, http.StatusOK, response(*d))
	})
}

func (a *ooohhAPI) setDialValue() http.Handler {
	type request struct {
		Token string   `json:"token"`
		Value *float64 `json:"value,omitempty"`
	}
	type response ooohh.Dial

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ooohh.DialID(api.URLParam(r, "id"))

		var body request
		err := api.Decode(w, r, &body)
		if err != nil {
			api.Problem(w, r, "Validation Error", "Invalid JSON", http.StatusBadRequest)
			return
		}

		if body.Token == "" || body.Value == nil {
			api.Problem(w, r, "Validation Error", "Both `token` and `value` must be provided.", http.StatusBadRequest)
			return
		}

		err = a.s.SetDial(r.Context(), id, body.Token, *body.Value)
		if err != nil {
			if errors.Is(err, ooohh.ErrDialNotFound) {
				api.NotFound(w, r)
				return
			} else if errors.Is(err, ooohh.ErrUnauthorized) {
				api.Problem(w, r, "Unauthorized", "Invalid token", http.StatusUnauthorized)
				return
			}

			a.logger.Errorw("could not update dial", "err", err, "id", id)
			api.Problem(w, r, "Internal Server Error", "Could not update dial", http.StatusInternalServerError)
			return
		}

		d, err := a.s.GetDial(r.Context(), id)
		if err != nil {
			a.logger.Errorw("could not retrieve dial", "err", err, "id", id)
			api.Problem(w, r, "Internal Server Error", "Could not update dial", http.StatusInternalServerError)
			return
		}

		api.Respond(w, r, http.StatusOK, response(*d))
	})
}

func (a *ooohhAPI) createBoard() http.Handler {
	type request struct {
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	type response ooohh.Board

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body request
		err := api.Decode(w, r, &body)
		if err != nil {
			api.Problem(w, r, "Validation Error", "Invalid JSON", http.StatusBadRequest)
			return
		}

		if body.Name == "" || body.Token == "" {
			api.Problem(w, r, "Validation Error", "Both `name` and `token` must be provided.", http.StatusBadRequest)
			return
		}

		b, err := a.s.CreateBoard(r.Context(), body.Name, body.Token)
		if err != nil {
			a.logger.Errorw("could not create board", "err", err)
			api.Problem(w, r, "Internal Server Error", "Could not create board", http.StatusInternalServerError)
			return
		}

		api.Respond(w, r, http.StatusCreated, response(*b))
	})
}

func (a *ooohhAPI) getBoard() http.Handler {
	type response ooohh.Board

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ooohh.BoardID(api.URLParam(r, "id"))

		b, err := a.s.GetBoard(r.Context(), id)
		if err != nil {
			if errors.Is(err, ooohh.ErrBoardNotFound) {
				api.NotFound(w, r)
				return
			}

			a.logger.Errorw("could not retrieve board", "err", err, "id", id)
			api.Problem(w, r, "Internal Server Error", "Could not retrieve board", http.StatusInternalServerError)
			return
		}

		api.Respond(w, r, http.StatusOK, response(*b))
	})
}

func (a *ooohhAPI) setBoardDials() http.Handler {
	type request struct {
		Token string    `json:"token"`
		Dials *[]string `json:"dials,omitempty"`
	}
	type response ooohh.Board

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := ooohh.BoardID(api.URLParam(r, "id"))

		var body request
		err := api.Decode(w, r, &body)
		if err != nil {
			api.Problem(w, r, "Validation Error", "Invalid JSON", http.StatusBadRequest)
			return
		}

		if body.Token == "" || body.Dials == nil {
			api.Problem(w, r, "Validation Error", "Both `token` and `dials` must be provided.", http.StatusBadRequest)
			return
		}

		dials := make([]ooohh.DialID, len(*body.Dials))
		for i := range dials {
			dials[i] = ooohh.DialID((*body.Dials)[i])
		}

		err = a.s.SetBoard(r.Context(), id, body.Token, dials)
		if err != nil {
			if errors.Is(err, ooohh.ErrBoardNotFound) {
				api.NotFound(w, r)
				return
			} else if errors.Is(err, ooohh.ErrUnauthorized) {
				api.Problem(w, r, "Unauthorized", "Invalid token", http.StatusUnauthorized)
				return
			}

			a.logger.Errorw("could not update board", "err", err, "id", id)
			api.Problem(w, r, "Internal Server Error", "Could not update board", http.StatusInternalServerError)
			return
		}

		b, err := a.s.GetBoard(r.Context(), id)
		if err != nil {
			a.logger.Errorw("could not retrieve board", "err", err, "id", id)
			api.Problem(w, r, "Internal Server Error", "Could not update board", http.StatusInternalServerError)
			return
		}

		api.Respond(w, r, http.StatusOK, response(*b))
	})
}

func (a *ooohhAPI) slackCommand() http.Handler {
	type request struct {
		Command string
		Text    string
		UserID  string
		TeamID  string
	}
	type response struct {
		Type string `json:"response_type"`
		Text string `json:"text"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		err := r.ParseForm()
		if err != nil {
			a.logger.Errorw("could not parse form", "err", err)
			// Return with a 500 to tell slack that we couldn't process this request.
			api.Problem(w, r, "Invalid Request", "Could not parse form", http.StatusInternalServerError)
			return
		}

		body := request{
			Command: r.FormValue("command"),
			Text:    r.FormValue("text"),
			UserID:  r.FormValue("user_id"),
			TeamID:  r.FormValue("team_id"),
		}

		if body.Command == "" || body.UserID == "" || body.TeamID == "" {
			a.logger.Errorw("could not parse request", "body", body)
			// Return with a 500 to tell slack that we couldn't process this request.
			api.Problem(w, r, "Invalid Request", "Could not parse form values", http.StatusInternalServerError)
			return
		}

		// Check the command is indeed `/wtf`.
		if body.Command != "/wtf" {
			api.Respond(w, r, http.StatusOK, response{
				Type: "ephemeral",
				Text: "Not sure what you mean there, friend.",
			})
			return
		}

		t := strings.TrimSpace(body.Text)

		// Return a help string.
		if t == "help" {
			api.Respond(w, r, http.StatusOK, response{
				Type: "ephemeral",
				Text: "Use the following format to set a value: `/wtf value`",
			})
			return
		}

		// Parse text into a float64. Respond with message if not ok.
		value, err := strconv.ParseFloat(t, 64)
		if err != nil {
			api.Respond(w, r, http.StatusOK, response{
				Type: "ephemeral",
				Text: "Please supply a single number as your WTF level.",
			})
			return
		}

		// Check number isn't NaN value.
		if math.IsNaN(value) {
			api.Respond(w, r, http.StatusOK, response{
				Type: "ephemeral",
				Text: "Sneaky. Please supply a _number_ as your WTF level.",
			})
			return
		}

		// Check number isn't infinite.
		if math.IsInf(value, 1) || math.IsInf(value, -1) {
			api.Respond(w, r, http.StatusOK, response{
				Type: "ephemeral",
				Text: "Definitely seek out help! Unfortunately, I only go up to 100.",
			})
			return
		}

		// Set value.
		err = a.ss.SetDialValue(r.Context(), body.TeamID, body.UserID, value)
		if err != nil {
			api.Respond(w, r, http.StatusOK, response{
				Type: "ephemeral",
				Text: "Oops, something didn't quite work out. Please, try again.",
			})
			return
		}

		// Calculate response text.
		text := "Ooohh, I wish I felt like that."
		if value > 75 {
			text = "Ooohh, make sure you check in with someone, maybe they can help."
		} else if value > 50 {
			text = "Ooohh, make sure you take a break!"
		}

		// Respond with ok.
		api.Respond(w, r, http.StatusOK, response{
			Type: "ephemeral",
			Text: text,
		})
	})
}
