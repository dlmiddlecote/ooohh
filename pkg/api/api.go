package api

import (
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
			Handler: nil,
		},
		{
			Method:  "PATCH",
			Path:    "/api/dials/:id",
			Handler: nil,
		},
		{
			Method:  "POST",
			Path:    "/api/boards",
			Handler: nil,
		},
		{
			Method:  "GET",
			Path:    "/api/boards/:id",
			Handler: nil,
		},
		{
			Method:  "PATCH",
			Path:    "/api/boards/:id",
			Handler: nil,
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
