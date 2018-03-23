package foo

import "github.com/samsarahq/go/oops"

type SnazzyError struct {
}

func NewSnazzyError() *SnazzyError {
	// Should not be wrapped as error is not assignale to *SnazzyError.
	return &SnazzyError{}
}

func (s *SnazzyError) Error() string {
	return "I am quite snazzy"
}

func f() (interface{}, error) {
	return nil, oops.Wrapf(NewSnazzyError(), "")
}
