package oops_test

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"testing"

	"github.com/samsarahq/go/oops"
	"github.com/stretchr/testify/assert"
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

func runWithRecover(f func()) (err error) {
	defer func() {
		err = oops.Recover(recover())
	}()
	f()
	return
}

type wrapperErr struct {
	inner error
}

func (e *wrapperErr) Unwrap() error {
	return e.inner
}

func (e *wrapperErr) Error() string {
	return "wrapper"
}

type baseErr struct{}

func (e *baseErr) Error() string {
	return "base"
}

func oopsChain() error {
	base := &baseErr{}
	a := oops.Wrapf(base, "a")
	b := oops.Wrapf(a, "b")
	middle := &wrapperErr{inner: b}
	c := oops.Wrapf(middle, "c")
	return oops.Wrapf(c, "d")
}

func chain() error {
	base := &baseErr{}
	a := oops.Wrapf(base, "a")
	b := oops.Wrapf(a, "b")
	return &wrapperErr{inner: b}
}

func TestRecoverNil(t *testing.T) {
	err := runWithRecover(func() {
		// Everything's fine!
	})
	if err != nil {
		t.Error(err)
	}
}

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
		{
			Title: "panic nil deref",
			Error: runWithRecover(func() {
				var i *int
				*i = 0
			}),
			Short: "runtime error: invalid memory address or nil pointer dereference",
			Verbose: `runtime error: invalid memory address or nil pointer dereference

github.com/samsarahq/go/oops_test.runWithRecover.func1: recovered panic
	github.com/samsarahq/go/oops/oops_test.go:123
runtime.call32
	runtime/asm_amd64.s:123
runtime.gopanic
	runtime/panic.go:123
runtime.panicmem
	runtime/panic.go:123
runtime.sigpanic
	runtime/signal_unix.go:123
github.com/samsarahq/go/oops_test.TestErrors.func1
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.runWithRecover
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
		{
			Title: "panic string",
			Error: runWithRecover(func() {
				panic("bad")
				var i *int
				*i = 0
			}),
			Short: "recovered panic: bad",
			Verbose: `recovered panic: bad

github.com/samsarahq/go/oops_test.runWithRecover.func1
	github.com/samsarahq/go/oops/oops_test.go:123
runtime.call32
	runtime/asm_amd64.s:123
runtime.gopanic
	runtime/panic.go:123
github.com/samsarahq/go/oops_test.TestErrors.func2
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.runWithRecover
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
		{
			Title: "panic error",
			Error: runWithRecover(func() {
				panic(errors.New("uh oh"))
			}),
			Short: "uh oh",
			Verbose: `uh oh

github.com/samsarahq/go/oops_test.runWithRecover.func1: recovered panic
	github.com/samsarahq/go/oops/oops_test.go:123
runtime.call32
	runtime/asm_amd64.s:123
runtime.gopanic
	runtime/panic.go:123
github.com/samsarahq/go/oops_test.TestErrors.func3
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.runWithRecover
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
		{
			Title: "panic oops",
			Error: runWithRecover(func() {
				panic(oops.Errorf("help!"))
			}),
			Short: "help!",
			Verbose: `help!

github.com/samsarahq/go/oops_test.TestErrors.func4
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.runWithRecover
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123

github.com/samsarahq/go/oops_test.runWithRecover.func1: recovered panic
	github.com/samsarahq/go/oops/oops_test.go:123
runtime.call32
	runtime/asm_amd64.s:123
runtime.gopanic
	runtime/panic.go:123
github.com/samsarahq/go/oops_test.TestErrors.func4
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.runWithRecover
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
		{
			Title: "chain oops",
			Error: oopsChain(),
			Short: "wrapper",
			Verbose: `base

github.com/samsarahq/go/oops_test.oopsChain: a
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123

github.com/samsarahq/go/oops_test.oopsChain: b
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123

github.com/samsarahq/go/oops_test.oopsChain: c
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123

github.com/samsarahq/go/oops_test.oopsChain: d
	github.com/samsarahq/go/oops/oops_test.go:123
github.com/samsarahq/go/oops_test.TestErrors
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
	}

	re := regexp.MustCompile(`\.(go|s):\d+`)
	for _, testcase := range testcases {
		actualVerbose := re.ReplaceAllString(fmt.Sprint(testcase.Error), `.$1:123`)
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

func TestFrames(t *testing.T) {
	testCases := []struct {
		description string
		err         error
		expected    [][]oops.Frame
	}{
		{
			description: "non-oops chain",
			err:         chain(),
			expected:    [][]oops.Frame{[]oops.Frame{oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.chain", Line: 109, Reason: "a"}, oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 471, Reason: ""}, oops.Frame{File: "testing/testing.go", Function: "testing.tRunner", Line: 827, Reason: ""}}, []oops.Frame{oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.chain", Line: 110, Reason: "b"}, oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 471, Reason: ""}, oops.Frame{File: "testing/testing.go", Function: "testing.tRunner", Line: 827, Reason: ""}}},
		},
		{
			description: "oops chain",
			err:         oopsChain(),
			expected:    [][]oops.Frame{[]oops.Frame{oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.oopsChain", Line: 100, Reason: "a"}, oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 476, Reason: ""}, oops.Frame{File: "testing/testing.go", Function: "testing.tRunner", Line: 827, Reason: ""}}, []oops.Frame{oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.oopsChain", Line: 101, Reason: "b"}, oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 476, Reason: ""}, oops.Frame{File: "testing/testing.go", Function: "testing.tRunner", Line: 827, Reason: ""}}, []oops.Frame{oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.oopsChain", Line: 103, Reason: "c"}, oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 476, Reason: ""}, oops.Frame{File: "testing/testing.go", Function: "testing.tRunner", Line: 827, Reason: ""}}, []oops.Frame{oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.oopsChain", Line: 104, Reason: "d"}, oops.Frame{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 476, Reason: ""}, oops.Frame{File: "testing/testing.go", Function: "testing.tRunner", Line: 827, Reason: ""}}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			assert.Equal(t, tc.expected, oops.Frames(tc.err))
		})
	}
}

func TestCause(t *testing.T) {
	base := &baseErr{}
	a := oops.Wrapf(base, "a")
	b := oops.Wrapf(a, "b")
	middle := &wrapperErr{inner: b}
	c := oops.Wrapf(middle, "c")
	d := oops.Wrapf(c, "d")

	assert.Equal(t, base, oops.Cause(base))
	assert.Equal(t, base, oops.Cause(a))
	assert.Equal(t, base, oops.Cause(b))

	assert.Equal(t, middle, oops.Cause(middle))
	assert.Equal(t, middle, oops.Cause(c))
	assert.Equal(t, middle, oops.Cause(d))
}

func TestIs(t *testing.T) {
	base := errors.New("base")
	a := oops.Wrapf(base, "a")
	b := oops.Wrapf(a, "b")
	middle := &wrapperErr{inner: b}
	c := oops.Wrapf(middle, "c")
	d := oops.Wrapf(c, "d")

	assert.True(t, oops.Is(a, base))
	assert.True(t, oops.Is(b, base))
	assert.True(t, oops.Is(middle, base))
	assert.True(t, oops.Is(c, base))
	assert.True(t, oops.Is(d, base))
}

func TestAs(t *testing.T) {
	base := &baseErr{}
	a := oops.Wrapf(base, "a")
	b := oops.Wrapf(a, "b")
	middle := &wrapperErr{inner: b}
	c := oops.Wrapf(middle, "c")
	d := oops.Wrapf(c, "d")

	var checkBase *baseErr
	assert.True(t, oops.As(a, &checkBase))
	assert.Equal(t, base, checkBase)

	checkBase = nil
	assert.True(t, oops.As(b, &checkBase))
	assert.Equal(t, base, checkBase)

	var checkWrapper *wrapperErr
	assert.True(t, oops.As(c, &checkWrapper))
	assert.Equal(t, middle, checkWrapper)

	checkWrapper = nil
	assert.True(t, oops.As(d, &checkWrapper))
	assert.Equal(t, middle, checkWrapper)
}
