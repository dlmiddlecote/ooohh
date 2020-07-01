package mock

import (
	"context"

	"github.com/dlmiddlecote/ooohh"
)

// Service provides a mock ooohh.Service.
type Service struct {
	CreateDialFn      func(ctx context.Context, name string, token string) (*ooohh.Dial, error)
	CreateDialInvoked bool

	GetDialFn      func(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error)
	GetDialInvoked bool

	SetDialFn      func(ctx context.Context, id ooohh.DialID, token string, value float64) error
	SetDialInvoked bool

	CreateBoardFn      func(ctx context.Context, name string, token string) (*ooohh.Board, error)
	CreateBoardInvoked bool

	GetBoardFn      func(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error)
	GetBoardInvoked bool

	SetBoardFn      func(ctx context.Context, id ooohh.BoardID, token string, dials []ooohh.DialID) error
	SetBoardInvoked bool
}

// CreateDial will create the dial with the given name,
// and associate it to the specified token.
func (s *Service) CreateDial(ctx context.Context, name string, token string) (*ooohh.Dial, error) {
	s.CreateDialInvoked = true
	return s.CreateDialFn(ctx, name, token)
}

// GetDial retrieves a dial by ID. Anyone can retrieve any dial with its ID.
func (s *Service) GetDial(ctx context.Context, id ooohh.DialID) (*ooohh.Dial, error) {
	s.GetDialInvoked = true
	return s.GetDialFn(ctx, id)
}

// SetDial updates the dial value. It can be updated by anyone who knows
// the original token it was created with.
func (s *Service) SetDial(ctx context.Context, id ooohh.DialID, token string, value float64) error {
	s.SetDialInvoked = true
	return s.SetDialFn(ctx, id, token, value)
}

// CreateBoard will create a board with the given name,
// and associate it to the specified token.
func (s *Service) CreateBoard(ctx context.Context, name string, token string) (*ooohh.Board, error) {
	s.CreateBoardInvoked = true
	return s.CreateBoardFn(ctx, name, token)
}

// GetBoard retrieves a board by ID. Anyone can retrieve any board with its ID.
func (s *Service) GetBoard(ctx context.Context, id ooohh.BoardID) (*ooohh.Board, error) {
	s.GetBoardInvoked = true
	return s.GetBoardFn(ctx, id)
}

// SetBoard updates the dials associated with the board. It can be updated
// by anyone who knows the original token it was created with.
func (s *Service) SetBoard(ctx context.Context, id ooohh.BoardID, token string, dials []ooohh.DialID) error {
	s.SetBoardInvoked = true
	return s.SetBoardFn(ctx, id, token, dials)
}

// SlackService provides a mock slack.Service.
type SlackService struct {
	SetDialValueFn      func(ctx context.Context, teamID, userID string, value float64) error
	SetDialValueInvoked bool
}

// SetDialValue updates the given user's dial value.
func (s *SlackService) SetDialValue(ctx context.Context, teamID string, userID string, value float64) error {
	s.SetDialValueInvoked = true
	return s.SetDialValueFn(ctx, teamID, userID, value)
}
