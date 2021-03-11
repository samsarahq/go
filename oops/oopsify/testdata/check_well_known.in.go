package foo

import (
	"io"
)

func f() error {
	return nil
}

func g() {
	if err := f(); err != io.EOF {
	}
}
