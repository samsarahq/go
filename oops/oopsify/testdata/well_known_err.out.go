package foo

import (
	"errors"
	"github.com/samsarahq/go/oops"
)

var ErrBad = errors.New("bad")

func f() (interface{}, error) {
	return nil, oops.Wrapf(ErrBad, "")
}
