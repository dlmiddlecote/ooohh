package mock

import (
	"testing"

	"github.com/matryer/is"

	"github.com/dlmiddlecote/ooohh"
)

func TestMockServiceIsOoohhService(t *testing.T) {

	is := is.New(t)

	var i interface{} = &Service{}
	_, ok := i.(ooohh.Service)
	is.True(ok) // mock service is ooohh service.
}
