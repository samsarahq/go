package main

import (
	"fmt"

	"github.com/samsarahq/go/oops"
)

// Foo creates new errors using oops.Errorf.
func Foo(i int) error {
	if i > 10 {
		return oops.Errorf("%d is too large!", i)
	}
	return nil
}

// Legacy is old code that does not use oops.
func Legacy(i int) error {
	return Foo(i)
}

// Bar wraps errors using Wrapf.
func Bar() error {
	if err := Legacy(20); err != nil {
		return oops.Wrapf(err, "Legacy(20) didn't work")
	}
	return nil
}

// Go wraps errors using Wrapf after receiving one from a channel!
func Go() error {
	ch := make(chan error)

	go func() {
		ch <- Bar()
	}()

	return oops.Wrapf(<-ch, "goroutine had a problem")
}

func main() {
	if err := Go(); err != nil {
		fmt.Print(err)
	}
}
