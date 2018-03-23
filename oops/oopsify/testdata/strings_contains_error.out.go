package foo

import (
	"strings"
	"github.com/samsarahq/go/oops"
)

func f() error {
	return nil
}

func g() {
	if err := f(); err != nil && !strings.Contains(oops.Cause(err).Error(), "some string") {
	}
}
