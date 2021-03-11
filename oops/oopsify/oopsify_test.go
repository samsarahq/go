package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io/ioutil"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/fatih/astrewrite"
	"github.com/kylelemons/godebug/diff"
	"github.com/stretchr/testify/require"
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

func loadTestCases(t *testing.T) []testCase {
	files, err := ioutil.ReadDir("testdata/")
	require.NoError(t, err)

	var testCases []testCase

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".in.go") {
			name := strings.TrimSuffix(file.Name(), ".in.go")
			inputName := file.Name()
			outputName := name + ".out.go"

			inputBytes, err := ioutil.ReadFile(path.Join("testdata", inputName))
			require.NoError(t, err)
			outputBytes, _ := ioutil.ReadFile(path.Join("testdata", outputName))

			testCases = append(testCases, testCase{
				name:   name,
				input:  string(inputBytes),
				output: string(outputBytes),
			})
		}
	}

	return testCases
}

func do(t *testing.T, source string) string {
	o := &oopsify{
		info: types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Uses:  make(map[*ast.Ident]types.Object),
		},
		fset: token.NewFileSet(),
		errs: make(map[token.Pos]errinfo),
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

var rewriteSnapshots = flag.Bool("rewriteSnapshots", false, "rewrite .out.go files")

func TestOopsify(t *testing.T) {
	for _, testCase := range loadTestCases(t) {
		t.Run(testCase.name, func(t *testing.T) {
			oopsified := do(t, testCase.input)

			if *rewriteSnapshots {
				ioutil.WriteFile(path.Join("testdata", testCase.name+".out.go"), []byte(oopsified), 0644)
			} else if diff := diff.Diff(oopsified, testCase.output); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestOopsifyIdempotent(t *testing.T) {
	for _, testCase := range loadTestCases(t) {
		t.Run(testCase.name, func(t *testing.T) {
			once := do(t, testCase.input)
			twice := do(t, once)
			if diff := diff.Diff(once, twice); diff != "" {
				t.Error(diff)
			}
		})
	}
}
