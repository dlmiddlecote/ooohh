package api

import (
	"errors"
	"net/http"

	"github.com/dlmiddlecote/kit/api"
	"go.uber.org/zap"

	"github.com/dlmiddlecote/ooohh"
)

type ooohhAPI struct {
	logger *zap.SugaredLogger
	s      ooohh.Service
}

// NewAPI returns an implementation of api.API.
// The returned API exposes the given ooohh service as an HTTP API.
func NewAPI(logger *zap.SugaredLogger, s ooohh.Service) *ooohhAPI {
	return &ooohhAPI{logger, s}
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
