package foo

import (
	"errors"
)

func f() (interface{}, error) {
	return nil, errors.New("hello!! %s")
}
