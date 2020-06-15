package ooohh

import (
	"context"
	"time"
)

// Dial represents an ooohh, wtf level for a user.
// The token is defined by the user, and is used for some simple authorization.
type Dial struct {
	ID        string    `json:"id"`
	Token     string    `json:"-"`
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Board represents a collection of Dials to be displayed together.
// The token is defined by the user, and is used for some simple authorization.
type Board struct {
	ID        string    `json:"id"`
	Token     string    `json:"-"`
	Name      string    `json:"name"`
	Dials     []Dial    `json:"dials"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Service represents a service for managing dials and boards
type Service interface {
	// CreateDial will create the dial with the given name,
	// and associate it to the specified token.
	CreateDial(ctx context.Context, name, token string) (*Dial, error)
	// GetDial retrieves a dial by ID. Anyone can retrieve any dial with its ID.
	GetDial(ctx context.Context, id string) (*Dial, error)
	// SetDial updates the dial value. It can be updated by anyone who knows
	// the original token it was created with.
	SetDial(ctx context.Context, id, token string, value float64) error

	// CreateBoard will create a board with the given name,
	// and associate it to the specified token.
	CreateBoard(ctx context.Context, name, token string) (*Board, error)
	// GetBoard retrieves a board by ID. Anyone can retrieve any board with its ID.
	GetBoard(ctx context.Context, id string) (*Board, error)
	// SetBoard updates the dials associated with the board. It can be updated
	// by anyone who knows the original token it was created with.
	SetBoard(ctx context.Context, id, token string, dials []Dial) error
}

//
// Errors
//

const (
	// ErrUnauthorized signifies the token is unauthorized to perform the attempted action
	ErrUnauthorized = Error("unauthorized")
	// ErrDialNotFound signifies that the dial specified is not found
	ErrDialNotFound = Error("dial not found")
	// ErrBoardNotFound signifies that the board specified is not found
	ErrBoardNotFound = Error("board not found")
)

// Error represents a ooohh, wtf error.
type Error string

// Error returns the error message, to implement the error interface.
func (e Error) Error() string {
	return string(e)
}
