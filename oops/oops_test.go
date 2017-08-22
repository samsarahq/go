package oops_test

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"testing"

	"github.com/samsarahq/go/oops"
)

func z() error {
	return oops.Wrapf(io.EOF, "reading failed")
}

func y() error {
	return oops.Wrapf(z(), "i guess some IO went wrong")
}

func c() error {
	return oops.Errorf("problem in c: %d", 10)
}

func b() error {
	return oops.Wrapf(c(), "b failed too")
}

func a() error {
	return oops.Wrapf(b(), "no no no")
}

func bb() error {
	ch := make(chan error)
	go func() {
		ch <- oops.Wrapf(a(), "causing trouble")
	}()
	return oops.Wrapf(<-ch, "bb had a bad time")
}

func aa() error {
	return oops.Wrapf(bb(), "aa didn't quite work out")
}

func rec(n int) error {
	if n > 0 {
		return oops.Wrapf(rec(n-1), "recursion %d", n)
	}
	return a()
}

func gap(n int) error {
	if n > 0 && n%2 == 0 {
		return oops.Wrapf(gap(n-1), "gap %d", n)
	} else if n > 0 {
		return gap(n - 1)
	}
	return oops.Wrapf(a(), "gap 0")
}

func rc() error {
	return oops.Wrapf(rootCause, "something rooty")
}

func doubleWrapf() error {
	return oops.Wrapf(oops.Wrapf(oops.Wrapf(rootCause, "yuck"), "bad"), "why would you do this")
}

var rootCause = errors.New("some root cause")

func TestErrors(t *testing.T) {
	testcases := []struct {
		Title   string
		Error   error
		Cause   error
		Short   string
		Verbose string
	}{
		{
			Title: "Existing",
			Error: y(),
			Short: "EOF",
			Cause: io.EOF,
			Verbose: `EOF

github.com/samsarahq/go/oops_test.z: reading failed
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.y: i guess some IO went wrong
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
		{
			Title: "Const root cause",
			Error: rc(),
			Short: "some root cause",
			Cause: rootCause,
			Verbose: `some root cause

github.com/samsarahq/go/oops_test.rc: something rooty
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
		{
			Title: "double wrap",
			Error: doubleWrapf(),
			Short: "some root cause",
			Cause: rootCause,
			Verbose: `some root cause

github.com/samsarahq/go/oops_test.doubleWrapf: yuck
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123

github.com/samsarahq/go/oops_test.doubleWrapf: bad
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123

github.com/samsarahq/go/oops_test.doubleWrapf: why would you do this
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},

		{
			Title: "Basic",
			Error: a(),
			Short: "problem in c: 10",
			Verbose: `problem in c: 10

github.com/samsarahq/go/oops_test.c
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.b: b failed too
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.a: no no no
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
		{
			Title: "Goroutines",
			Error: aa(),
			Short: "problem in c: 10",
			Verbose: `problem in c: 10

github.com/samsarahq/go/oops_test.c
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.b: b failed too
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.a: no no no
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.bb.func1: causing trouble
	github.com/samsarahq/go/oops/oops_test.go:123

github.com/samsarahq/go/oops_test.bb: bb had a bad time
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.aa: aa didn't quite work out
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
		{
			Title: "Recursive",
			Error: rec(5),
			Short: "problem in c: 10",
			Verbose: `problem in c: 10

github.com/samsarahq/go/oops_test.c
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.b: b failed too
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.a: no no no
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.rec
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.rec: recursion 1
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.rec: recursion 2
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.rec: recursion 3
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.rec: recursion 4
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.rec: recursion 5
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
		{
			Title: "Gap",
			Error: gap(5),
			Short: "problem in c: 10",
			Verbose: `problem in c: 10

github.com/samsarahq/go/oops_test.c
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.b: b failed too
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.a: no no no
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.gap: gap 0
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.gap
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.gap: gap 2
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.gap
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.gap: gap 4
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.gap
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
	}

	re := regexp.MustCompile(`\.go:\d+`)
	for _, testcase := range testcases {
		actualVerbose := re.ReplaceAllString(fmt.Sprint(testcase.Error), ".go:123")
		if actualVerbose != testcase.Verbose {
			t.Errorf("verbose %s:\nexpected:\n%s\nactual:\n%s", testcase.Title, testcase.Verbose, actualVerbose)
		}

		actualShort := oops.Cause(testcase.Error).Error()
		if actualShort != testcase.Short {
			t.Errorf("short %s:\nexpected:\n%s\nactual:\n%s", testcase.Title, testcase.Short, actualShort)
		}

		actualCause := oops.Cause(testcase.Error)
		if testcase.Cause != nil && testcase.Cause != actualCause {
			t.Errorf("root cause %s:\nexpected:\n%v\nactual:\n%v", testcase.Title, testcase.Cause, actualCause)
		}
	}
}
