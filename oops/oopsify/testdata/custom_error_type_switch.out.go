package foo

import "github.com/samsarahq/go/oops"

type SnazzyError struct {
}

func (s *SnazzyError) Error() string {
	return "I am quite snazzy"
}

func f() error {
	return nil
}

func g() {
	if err := f(); err != nil {
		switch oops.Cause(err).(type) {
		}
	}
}
