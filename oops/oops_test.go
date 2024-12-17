package oops_test

import (
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"runtime"
	"sort"
	"strings"
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

func stripPathPrefix(s string) (string, error) {
	// Strip path prefixes from filenames, aids comparisons in stack trace tests
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot determine caller filename")
	}
	pth := path.Dir(filename)
	return strings.ReplaceAll(s, pth, "github.com/samsarahq/go/oops"), nil

}

func fixLineNumbers(s string) string {
	// Standardize line numbers in captured stack traces. These can vary depending on the go version in use at runtime, also
	// because the tests often reference LOC in this file, then additions / deletions to this file require tedious
	// refactoring on expected results.
	re := regexp.MustCompile(`(?m)\.(go|s):\d+$`)
	return re.ReplaceAllString(s, `.$1:123`)
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

func newNonOopsChainErr(prefix string, err error) *nonOopsChainErr {
	return &nonOopsChainErr{
		prefix: prefix,
		inner:  err,
	}
}

type nonOopsChainErr struct {
	prefix string
	inner  error
}

func (e *nonOopsChainErr) Error() string {
	return fmt.Sprintf("%s: %s", e.prefix, e.inner.Error())
}

func (e *nonOopsChainErr) Unwrap() error {
	return e.inner
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
		{
			Title: "oops wrapping nonOops wrapper",
			Error: oops.Wrapf(newNonOopsChainErr("somePrefix", errors.New("test error")), "oops err"),
			Short: "somePrefix: test error",
			Verbose: `somePrefix: test error

github.com/samsarahq/go/oops_test.TestErrors: oops err
	github.com/samsarahq/go/oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123
`,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.Title, func(t *testing.T) {
			actualVerbose := fixLineNumbers(fmt.Sprint(testcase.Error))
			actualVerbose, err := stripPathPrefix(actualVerbose)
			assert.NoError(t, err)
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
		})
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
			expected: [][]oops.Frame{
				{
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.chain", Line: 999, Reason: "a"},
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 999, Reason: ""},
					{File: "testing/testing.go", Function: "testing.tRunner", Line: 999, Reason: ""},
				},
				{
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.chain", Line: 999, Reason: "b"},
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 999, Reason: ""},
					{File: "testing/testing.go", Function: "testing.tRunner", Line: 999, Reason: ""},
				},
			},
		},
		{
			description: "oops chain",
			err:         oopsChain(),
			expected: [][]oops.Frame{
				{
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.oopsChain", Line: 999, Reason: "a"},
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 999, Reason: ""},
					{File: "testing/testing.go", Function: "testing.tRunner", Line: 999, Reason: ""},
				},
				{
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.oopsChain", Line: 999, Reason: "b"},
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 999, Reason: ""},
					{File: "testing/testing.go", Function: "testing.tRunner", Line: 999, Reason: ""},
				},
				{
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.oopsChain", Line: 999, Reason: "c"},
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 999, Reason: ""},
					{File: "testing/testing.go", Function: "testing.tRunner", Line: 999, Reason: ""}},
				{
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.oopsChain", Line: 999, Reason: "d"},
					{File: "github.com/samsarahq/go/oops/oops_test.go", Function: "github.com/samsarahq/go/oops_test.TestFrames", Line: 999, Reason: ""},
					{File: "testing/testing.go", Function: "testing.tRunner", Line: 999, Reason: ""},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {

			// we need to modify the returned errors to replace the actual paths to match the hard-coded test values
			frames := oops.Frames(tc.err)
			sanitizedFrames := make([][]oops.Frame, len(frames))
			for i, innerFrames := range frames {
				newInnerFrames := make([]oops.Frame, len(innerFrames))
				for j, actualFrame := range innerFrames {
					// assert that we capture line numbers, unreliable to assert they exactly equal a value though
					assert.Greater(t, actualFrame.Line, 0)
					frameFilename, err := stripPathPrefix(actualFrame.File)
					assert.NoError(t, err)
					newInnerFrames[j] = oops.Frame{File: frameFilename, Line: 999, Function: actualFrame.Function, Reason: actualFrame.Reason}
				}
				sanitizedFrames[i] = newInnerFrames
			}
			assert.Equal(t, tc.expected, sanitizedFrames)
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

func TestOopsSkipFrame(t *testing.T) {
	err := getTestError()
	newErr := oops.SkipFrames(err, 1)
	oldFrames := oops.Frames(err)

	newFrames := oops.Frames(newErr)
	assert.Equal(t, oldFrames[0][1:], newFrames[0])
}

func TestOopsSkipFrameWithInvalidInput(t *testing.T) {
	err := getTestError()
	oldFrames := oops.Frames(err)
	newErr := oops.SkipFrames(err, -1)
	newErr1 := oops.SkipFrames(err, 0)
	assert.Equal(t, oldFrames, oops.Frames(newErr))
	assert.Equal(t, oldFrames, oops.Frames(newErr1))
}

func TestOopsSkipMoreFramesThanExists(t *testing.T) {
	err := getTestError()
	oldFrames := oops.Frames(err)
	newErr := oops.SkipFrames(err, 10000)
	assert.Equal(t, oldFrames, oops.Frames(newErr))
}

func getTestError() error {
	return oops.Errorf("test")
}

type reasonErr interface {
	Reason() string
	Error() string
}

func TestReasonEmptyString(t *testing.T) {
	err := oops.Errorf("a").(reasonErr)
	err2 := oops.Wrapf(err, "").(reasonErr)
	assert.Equal(t, "", err.Reason())
	assert.Equal(t, "", err2.Reason())
}

func TestStackedOopsErrors(t *testing.T) {
	err := fmt.Errorf("Error")
	err = oops.Wrapf(err, "d")
	err = oops.Wrapf(err, "c")
	err = oops.Wrapf(err, "b")
	e := oops.Wrapf(err, "a").(reasonErr)
	assert.Equal(t, "a: b: c: d", e.Reason())
}

func TestPrintMainStack(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "oops chain",
			err:  oops.Errorf("test"),
			want: `test

github.com/samsarahq/go/oops_test.TestPrintMainStack
	github.com/samsarahq/go/oops/oops_test.go:XXX
testing.tRunner
	testing/testing.go:XXX
`,
		},
		{
			name: "oops chain",
			err:  chain(),
			want: `base

github.com/samsarahq/go/oops_test.chain: a
	github.com/samsarahq/go/oops/oops_test.go:XXX
github.com/samsarahq/go/oops_test.TestPrintMainStack
	github.com/samsarahq/go/oops/oops_test.go:XXX
testing.tRunner
	testing/testing.go:XXX
`,
		},
		{
			name: "empty error",
			err:  nil,
			want: "",
		},
		{
			name: "non-oops error",
			err:  &baseErr{},
			want: "",
		},
	}
	// replace digits by XXX so test is not affected by line numbers
	digitRegex := regexp.MustCompile("[0-9]+")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := oops.MainStackToString(tt.err)
			got = digitRegex.ReplaceAllString(got, "XXX")
			got, err := stripPathPrefix(got)
			assert.NoError(t, err)
			if got != tt.want {
				t.Errorf("MainStackToString() = \n%v, want: \n%v", got, tt.want)
			}

		})
	}
}

func BenchmarkErrorf(b *testing.B) {
	for n := 0; n < b.N; n++ {
		oops.Errorf("boom goes the dynamite")
	}
}

func BenchmarkWrapf(b *testing.B) {
	benchmarkCases := []struct {
		name string
		err  error
	}{
		{
			name: "nil error",
		},
		{
			name: "non-oops error",
			err:  errors.New("not great, bob!"),
		},
		{
			name: "direct oops error",
			err:  oops.Errorf("scusi"),
		},
		{
			name: "nested oops error",
			err:  oops.Wrapf(oops.Errorf("scusi"), "mea culpa"),
		},
		{
			name: "triply nested oops error",
			err:  oops.Wrapf(oops.Wrapf(oops.Errorf("scusi"), "mea culpa"), "did i do that"),
		},
	}

	b.ResetTimer()

	for _, bc := range benchmarkCases {
		b.Run(bc.name, func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				oops.Wrapf(bc.err, "boom goes the dynamite")
			}
		})
	}
}

func BenchmarkOopsErrorError(b *testing.B) {
	err := oops.Errorf("boom goes the dynamite")
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err.Error()
	}
}

func BenchmarkMainStackToString(b *testing.B) {
	err := oops.Errorf("boom goes the dynamite")
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		oops.MainStackToString(err)
	}
}

func TestPrefixesToShortCircuit(t *testing.T) {
	assert.Empty(t, oops.GetPrefixesToShortCircuit())

	oops.SetPrefixesToShortCircuit()
	assert.Empty(t, oops.GetPrefixesToShortCircuit())

	oops.SetPrefixesToShortCircuit("foo", "bar")
	prefixes := oops.GetPrefixesToShortCircuit()
	// Sort so test output is stable.
	sort.Strings(prefixes)
	assert.Equal(t, []string{"bar", "foo"}, prefixes)

	oops.SetPrefixesToShortCircuit("baz")
	assert.Equal(t, []string{"baz"}, oops.GetPrefixesToShortCircuit())
}

// getFileDirectory gets the directory of the file at the given call depth in the stack in relation to the function that
// invokes `getFileDirectory`.
func getFileDirectory(t *testing.T, callDepth int) string {
	_, filename, _, _ := runtime.Caller(callDepth + 1)
	lastIndex := strings.LastIndex(filename, "/")
	assert.NotEqual(t, -1, lastIndex)
	return filename[:lastIndex]
}

func TestErrorStringTruncation(t *testing.T) {
	err := oops.Errorf("not great, bob")
	// Gets the path of the test directory. Because this differs depending on the installation of go, we can't hardcode
	// the prefixes to short circuit.
	goTestDir := getFileDirectory(t, 1)
	oopsTestDir := getFileDirectory(t, 0)

	testCases := []struct {
		name                   string
		prefixesToShortCircuit []string
		expectedOutput         string
	}{
		{
			name:                   "no truncation",
			prefixesToShortCircuit: []string{},
			expectedOutput: `not great, bob

github.com/samsarahq/go/oops_test.TestErrorStringTruncation
	oops/oops_test.go:123
testing.tRunner
	testing/testing.go:123

`,
		},
		{
			name:                   "go testing package truncated",
			prefixesToShortCircuit: []string{goTestDir},
			expectedOutput: `not great, bob

github.com/samsarahq/go/oops_test.TestErrorStringTruncation
	oops/oops_test.go:123
subsequent stack frames truncated

`,
		},
		{
			name:                   "oops package truncated",
			prefixesToShortCircuit: []string{oopsTestDir},
			expectedOutput: `not great, bob

subsequent stack frames truncated

`,
		},
		{
			name:                   "oops and go testing packages truncated",
			prefixesToShortCircuit: []string{oopsTestDir, goTestDir},
			expectedOutput: `not great, bob

subsequent stack frames truncated

`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oops.SetPrefixesToShortCircuit(tc.prefixesToShortCircuit...)
			errText := err.Error()
			// Remove the content before 'oops/' and 'testing/' in lines that contain said strings so that the tests
			// for oops are portable.
			sanitizedErrText := stripPrecedingFromAllLines(errText, "oops/", "testing/")
			sanitizedErrText = fixLineNumbers(sanitizedErrText)
			assert.Equal(t, tc.expectedOutput, sanitizedErrText)
		})
	}
}

func TestCollectMetadata(t *testing.T) {
	err := errors.New("oops")
	err1 := oops.WrapfWithMetadata(err, map[string]interface{}{
		"a": "b",
	}, "err1")
	err2 := oops.WrapfWithMetadata(err1, map[string]interface{}{
		"c": 1,
	}, "err2")
	err3 := oops.WrapfWithMetadata(err1, map[string]interface{}{
		"a": 1,
		"d": "e",
	}, "err3")
	nonOopsErr := fmt.Errorf("wraps oops err: %w", err1)

	testCases := []struct {
		description string
		error       error
		want        map[string]interface{}
	}{
		{
			description: "metadata is retrieved from oops error",
			error:       err1,
			want: map[string]interface{}{
				"a": "b",
			},
		},
		{
			description: "metadata is aggregated from 2 oops errors",
			error:       err2,
			want: map[string]interface{}{
				"a": "b",
				"c": 1,
			},
		},
		{
			description: "metadata is aggregated from 2 oops errors and key deduplicated by selecting outer most value",
			error:       err3,
			want: map[string]interface{}{
				"a": 1,
				"d": "e",
			},
		},
		{
			description: "metadata is retrieved from non oops error wrapping an oops error",
			error:       nonOopsErr,
			want: map[string]interface{}{
				"a": "b",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			metadata := oops.CollectMetadata(tc.error)

			assert.Equal(t, tc.want, metadata)
		})
	}

}

// stripPrecedingFromAllLines strips the content in each line in errorString up until toStrip. It adds a '\t' character
// in each line that it strips.
func stripPrecedingFromAllLines(errorString string, toStrip ...string) string {
	splitByLines := strings.Split(errorString, "\n")
	var builder strings.Builder
	for _, line := range splitByLines {
		currentLine := line
		for _, strip := range toStrip {
			lastIndex := strings.LastIndex(currentLine, strip)
			if lastIndex != -1 {
				currentLine = "\t" + currentLine[lastIndex:]
			}
		}
		builder.WriteString(currentLine)
		builder.WriteRune('\n')
	}
	return builder.String()
}
