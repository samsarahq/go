package foo

import (
	"strings"
)

func f() error {
	return nil
}

func g() {
	if err := f(); err != nil && !strings.Contains(err.Error(), "some string") {
	}
}
