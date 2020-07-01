package mock

import (
	"testing"

	"github.com/matryer/is"

	"github.com/dlmiddlecote/ooohh"
	"github.com/dlmiddlecote/ooohh/pkg/slack"
)

func TestMockServiceIsOoohhService(t *testing.T) {

	is := is.New(t)

	var i interface{} = &Service{}
	_, ok := i.(ooohh.Service)
	is.True(ok) // mock service is ooohh service.
}

func TestMockSlackServiceIsSlackService(t *testing.T) {

	is := is.New(t)

	var i interface{} = &SlackService{}
	_, ok := i.(slack.Service)
	is.True(ok) // mock slack service is a slack service.
}
