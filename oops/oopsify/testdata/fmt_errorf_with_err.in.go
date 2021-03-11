package foo

import (
	"fmt"
)

func f() (interface{}, error) {
	var err error
	return nil, fmt.Errorf("some thing went wrong: %s", err)
}
