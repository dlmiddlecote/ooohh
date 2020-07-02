package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/dlmiddlecote/ooohh"
	"github.com/pkg/errors"
)

type client struct {
	base *url.URL
	c    *http.Client
}

type (
	createDialRequest struct {
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	setDialRequest struct {
		Token string  `json:"token"`
		Value float64 `json:"value"`
	}
	problemResponse struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
)

func NewClient(base *url.URL) *client {
	return &client{
		base: base,
		c: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CreateDial will create the dial with the given name,
// and associate it to the specified token.
func (c *client) CreateDial(ctx context.Context, name string, token string) (*ooohh.Dial, error) {

	rel := &url.URL{Path: "/api/dials"}
	u := c.base.ResolveReference(rel)

	b, err := json.Marshal(createDialRequest{
		Name:  name,
		Token: token,
	})
	if err != nil {
		return nil, errors.Wrap(err, "marshalling json for request")
	}

	r, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewBuffer(b))
	if err != nil {
		return nil, errors.Wrap(err, "creating request")
	}

	r.Header.Set("User-Agent", "ooohh cli")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")

	resp, err := c.c.Do(r)
	if err != nil {
		return nil, errors.Wrap(err, "making request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		// decode problem response
		var problem problemResponse
		err = json.NewDecoder(resp.Body).Decode(&problem)
		if err != nil {
			return nil, errors.Wrap(err, "invalid response")
		}
		return nil, errors.New(problem.Title)
	}

	// decode into a dial
	var d ooohh.Dial
	err = json.NewDecoder(resp.Body).Decode(&d)
	if err != nil {
		return nil, errors.Wrap(err, "invalid response")
	}

	return &d, nil
}

// GetDial retrieves a dial by ID. Anyone can retrieve any dial with its ID.
func (c *client) GetDial(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error) {

	rel := &url.URL{Path: path.Join("/api/dials", string(id))}
	u := c.base.ResolveReference(rel)

	r, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, errors.Wrap(err, "creating request")
	}

	r.Header.Set("User-Agent", "ooohh cli")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")

	resp, err := c.c.Do(r)
	if err != nil {
		return nil, errors.Wrap(err, "making request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// decode problem response
		var problem problemResponse
		err = json.NewDecoder(resp.Body).Decode(&problem)
		if err != nil {
			return nil, errors.Wrap(err, "invalid response")
		}
		return nil, errors.New(problem.Title)
	}

	// decode into a dial
	var d ooohh.Dial
	err = json.NewDecoder(resp.Body).Decode(&d)
	if err != nil {
		return nil, errors.Wrap(err, "invalid response")
	}

	return &d, nil
}

// SetDial updates the dial value. It can be updated by anyone who knows
// the original token it was created with.
func (c *client) SetDial(ctx context.Context, id ooohh.DialID, token string, value float64) error {

	rel := &url.URL{Path: path.Join("/api/dials", string(id))}
	u := c.base.ResolveReference(rel)

	b, err := json.Marshal(setDialRequest{
		Token: token,
		Value: value,
	})
	if err != nil {
		return errors.Wrap(err, "marshalling json for request")
	}

	r, err := http.NewRequestWithContext(ctx, "PATCH", u.String(), bytes.NewBuffer(b))
	if err != nil {
		return errors.Wrap(err, "creating request")
	}

	r.Header.Set("User-Agent", "ooohh cli")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")

	resp, err := c.c.Do(r)
	if err != nil {
		return errors.Wrap(err, "making request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// decode problem response
		var problem problemResponse
		err = json.NewDecoder(resp.Body).Decode(&problem)
		if err != nil {
			return errors.Wrap(err, "invalid response")
		}
		return errors.New(problem.Title)
	}

	return nil
}

// CreateBoard will create a board with the given name,
// and associate it to the specified token.
func (c *client) CreateBoard(ctx context.Context, name string, token string) (*ooohh.Board, error) {
	panic("not implemented") // TODO: Implement
}

// GetBoard retrieves a board by ID. Anyone can retrieve any board with its ID.
func (c *client) GetBoard(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
	panic("not implemented") // TODO: Implement
}

// SetBoard updates the dials associated with the board. It can be updated
// by anyone who knows the original token it was created with.
func (c *client) SetBoard(ctx context.Context, id ooohh.BoardID, token string, dials []ooohh.DialID) error {
	panic("not implemented") // TODO: Implement
}
