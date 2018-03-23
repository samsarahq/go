package foo

import (
	"github.com/samsarahq/go/oops"
)

func f() (interface{}, error) {
	return nil, oops.Errorf("hello!!")
}
