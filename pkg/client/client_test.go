package client

import (
	"testing"

	"github.com/dlmiddlecote/ooohh"
	"github.com/matryer/is"
)

func TestClientIsOoohhService(t *testing.T) {

	is := is.New(t)

	var i interface{} = &client{}
	_, ok := i.(ooohh.Service)
	is.True(ok) // client is ooohh service.
}
