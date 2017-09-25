package main

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strconv"
	"testing"

	"github.com/fatih/astrewrite"
	"github.com/kylelemons/godebug/diff"
	"golang.org/x/tools/go/ast/astutil"
)

// fixupImports imports oops if necessary and removes unused imports. It's not
// as good as goimports, but suffices for the tests.
func (o *oopsify) fixupImports(file *ast.File) {
	astutil.AddImport(o.fset, file, "github.com/samsarahq/go/oops")
	for _, imports := range astutil.Imports(o.fset, file) {
		for _, i := range imports {
			v, _ := strconv.Unquote(i.Path.Value)
			if !astutil.UsesImport(file, v) {
				astutil.DeleteImport(o.fset, file, v)
			}
		}
	}
}

type testCase struct {
	name   string
	input  string
	output string
}

var testCases = []testCase{
	{
		name: "empty package",
		input: `package foo
`,
		output: `package foo
`,
	},
	{
		name: "simple errors.New",
		input: `package foo

import (
	"errors"
)

func f() (interface{}, error) {
	return nil, errors.New("hello!!")
}
`,
		output: `package foo

import (
	"github.com/samsarahq/go/oops"
)

func f() (interface{}, error) {
	return nil, oops.Errorf("hello!!")
}
`,
	},
	{
		name: "errors.New escape format",
		input: `package foo

import (
	"errors"
)

func f() (interface{}, error) {
	return nil, errors.New("hello!! %s")
}
`,
		output: `package foo

import (
	"github.com/samsarahq/go/oops"
)

func f() (interface{}, error) {
	return nil, oops.Errorf("hello!! %%s")
}
`,
	},
	{
		name: "errors.New complex",
		input: `package foo

import (
	"errors"
)

func g() string {
	return ""
}

func f() (interface{}, error) {
	return nil, errors.New(g())
}
`,
		output: `package foo

import (
	"github.com/samsarahq/go/oops"
)

func g() string {
	return ""
}

func f() (interface{}, error) {
	return nil, oops.Errorf("%s", g())
}
`,
	},
	{
		name: "simple fmt.Errorf",
		input: `package foo

import (
	"fmt"
)

func f() (interface{}, error) {
	return nil, fmt.Errorf("some thing went wrong %s %d", "foo", 10)
}
`,
		output: `package foo

import (
	"github.com/samsarahq/go/oops"
)

func f() (interface{}, error) {
	return nil, oops.Errorf("some thing went wrong %s %d", "foo", 10)
}
`,
	},
	{
		name: "fmt.Errorf with %s error",
		input: `package foo

import (
	"fmt"
)

func f() (interface{}, error) {
	var err error
	return nil, fmt.Errorf("some thing went wrong: %s", err)
}
`,
		output: `package foo

import (
	"github.com/samsarahq/go/oops"
)

func f() (interface{}, error) {
	var err error
	return nil, oops.Wrapf(err, "some thing went wrong")
}
`,
	},
	{
		name: "wrap global error",
		input: `package foo

import (
	"errors"
)

var ErrBad = errors.New("bad")

func f() (interface{}, error) {
	return nil, ErrBad
}
`,
		output: `package foo

import (
	"errors"
	"github.com/samsarahq/go/oops"
)

var ErrBad = errors.New("bad")

func f() (interface{}, error) {
	return nil, oops.Wrapf(ErrBad, "")
}
`,
	},
	{
		name: "wrap custom error type",
		input: `package foo

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
	return nil, NewSnazzyError()
}
`,
		output: `package foo

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
`,
	},
	{
		name: "check nil",
		input: `package foo

func f() error {
	return nil
}

func g() {
	if err := f(); err == nil {
	}
}
`,
		output: `package foo

func f() error {
	return nil
}

func g() {
	if err := f(); err == nil {
	}
}
`,
	},
	{
		name: "check non-nil",
		input: `package foo

func f() error {
	return nil
}

func g() {
	if err := f(); err != nil {
	}
}
`,
		output: `package foo

func f() error {
	return nil
}

func g() {
	if err := f(); err != nil {
	}
}
`,
	},
	{
		name: "check well known",
		input: `package foo

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
`,
		output: `package foo

import (
	"io"
	"github.com/samsarahq/go/oops"
)

func f() error {
	return nil
}

func g() {
	if err := f(); oops.Cause(err) != io.EOF {
	}
}
`,
	},
	{
		name: "error strings.Contains",
		input: `package foo

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
`,
		output: `package foo

import (
	"strings"
	"github.com/samsarahq/go/oops"
)

func f() error {
	return nil
}

func g() {
	if err := f(); err != nil && !strings.Contains(oops.Cause(err).Error(), "some string") {
	}
}
`,
	},
	{
		name: "normal strings.Contains",
		input: `package foo

import (
	"strings"
)

func f() bool {
	return strings.Contains("foo", "bar")
}
`,
		output: `package foo

import (
	"strings"
)

func f() bool {
	return strings.Contains("foo", "bar")
}
`,
	},
	{
		name: "assert custom error type",
		input: `package foo

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
		if _, ok := err.(*SnazzyError); ok {
		}
	}
}
`,
		output: `package foo

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
		if _, ok := oops.Cause(err).(*SnazzyError); ok {
		}
	}
}
`,
	},
	{
		name: "switch custom error type",
		input: `package foo

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
		switch err.(type) {
		}
	}
}
`,
		output: `package foo

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
`,
	},
}

func do(t *testing.T, source string) string {
	o := &oopsify{
		info: types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Uses:  make(map[*ast.Ident]types.Object),
		},
		fset: token.NewFileSet(),
	}

	file, err := parser.ParseFile(o.fset, "main.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	conf := types.Config{
		Importer: importer.Default(),
	}

	_, err = conf.Check("main", o.fset, []*ast.File{file}, &o.info)
	if err != nil {
		t.Fatal(err)
	}

	os := &oopsifyState{
		oopsify: o,
	}
	file = astrewrite.Walk(file, os.rewrite).(*ast.File)

	o.fixupImports(file)

	astPrinter := printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: 8,
	}

	var buffer bytes.Buffer
	astPrinter.Fprint(&buffer, o.fset, file)
	return buffer.String()
}

func TestOopsify(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			oopsified := do(t, testCase.input)
			if diff := diff.Diff(oopsified, testCase.output); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestOopsifyIdempotent(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			once := do(t, testCase.input)
			twice := do(t, once)
			if diff := diff.Diff(once, twice); diff != "" {
				t.Error(diff)
			}
		})
	}
}
