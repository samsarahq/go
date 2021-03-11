package foo

import (
	"fmt"
)

func f() (interface{}, error) {
	return nil, fmt.Errorf("some thing went wrong %s %d", "foo", 10)
}
