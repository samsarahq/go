package foo

import (
	"github.com/samsarahq/go/oops"
)

func g() string {
	return ""
}

func f() (interface{}, error) {
	return nil, oops.Errorf("%s", g())
}
