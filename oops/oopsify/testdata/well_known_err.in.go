package foo

import (
	"errors"
)

var ErrBad = errors.New("bad")

func f() (interface{}, error) {
	return nil, ErrBad
}
