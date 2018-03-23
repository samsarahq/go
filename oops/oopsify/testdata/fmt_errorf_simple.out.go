package foo

import (
	"github.com/samsarahq/go/oops"
)

func f() (interface{}, error) {
	return nil, oops.Errorf("some thing went wrong %s %d", "foo", 10)
}
