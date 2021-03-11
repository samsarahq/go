package foo

import (
	"github.com/samsarahq/go/oops"
)

func f() (interface{}, error) {
	var err error
	return nil, oops.Wrapf(err, "some thing went wrong")
}
