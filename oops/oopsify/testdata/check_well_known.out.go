package foo

import (
	"io"
	"github.com/samsarahq/go/oops"
)

func f() error {
	return nil
}

func g() {
	if err := f(); oops.Cause(err) != io.EOF {
	}
}
