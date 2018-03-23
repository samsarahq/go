package foo

import (
	"errors"
)

func g() string {
	return ""
}

func f() (interface{}, error) {
	return nil, errors.New(g())
}
