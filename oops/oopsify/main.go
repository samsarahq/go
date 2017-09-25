package main

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/fatih/astrewrite"
)

// errorType holds the built-in type "error".
var errorType *types.Interface

func init() {
	errorType = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
}

// oopsify holds the global state for an oopsify run.
type oopsify struct {
	fset *token.FileSet
	info types.Info
}

// oopsifyState holds the local state for an oopsify run.
type oopsifyState struct {
	*oopsify
	// function is the enclosing function type, useful to check return types.
	function *ast.FuncType
}

// funcName returns the fully-qualified function name of n, formatted as
// github.com/samsarahq/go/oops.Errorf. If n is not a function call, funcName
// returns an empty string.
func (o *oopsify) funcName(n ast.Expr) string {
	callExpr, ok := n.(*ast.CallExpr)
	if !ok {
		return ""
	}

	selectorExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}

	f, ok := o.info.Uses[selectorExpr.Sel].(*types.Func)
	if !ok {
		return ""
	}

	fullName := f.FullName()
	if idx := strings.LastIndex(fullName, "vendor/"); idx != -1 {
		fullName = fullName[idx+len("vendor/"):]
	}
	return fullName
}

// rewriteReturnStmt rewrites a return stmt to wrap errors with oops.Wrapf. It
// handles existing and globals errors, errors.New, and fmt.Errorf.
//
// We wrap errors at return statements (and not, say, when assiging to a local
// variable, or calling another function with an error argument) because only at
// the return statement we can be 99% confident that the error is being passed
// to the parent and would benefit from a stack trace.
func (o *oopsifyState) rewriteReturnStmt(n *ast.ReturnStmt) (ast.Node, bool) {
	// Check all return values.
	for i, res := range n.Results {
		// Bail out if we are not returning an error value.
		// XXX: nil in some _test file returning a big struct
		if o.info.Types[res].Type == nil || !types.Implements(o.info.Types[res].Type, errorType) {
			continue
		}

		// Bail out if the function signature has a type more specific than error (eg.
		// a custom Error struct.)
		if !types.AssignableTo(errorType, o.info.Types[o.function.Results.List[i].Type].Type) {
			continue
		}

		// Determine how to wrap the value.
		switch o.funcName(res) {
		case "github.com/samsarahq/go/oops.Errorf", "github.com/samsarahq/go/oops.Wrapf":
			// Don't do anything with existing oops calls.

		case "errors.New":
			// Rewrite errors.New("foo") to oops.Errorf("foo"), escaping the string argument.
			callExpr := res.(*ast.CallExpr)
			callExpr.Fun = &ast.SelectorExpr{
				X:   ast.NewIdent("oops"),
				Sel: ast.NewIdent("Errorf"),
			}

			if value := o.info.Types[callExpr.Args[0]].Value; value != nil {
				// For a constant string, escape all % signs.
				callExpr.Args = []ast.Expr{
					&ast.BasicLit{
						Value: strconv.Quote(strings.Replace(constant.StringVal(value), "%", "%%", -1)),
					},
				}
			} else {
				// For a dynamically computed string x, rewrite to oops.Errorf("%s", x).
				callExpr.Args = []ast.Expr{
					&ast.BasicLit{Kind: token.STRING, Value: `"%s"`},
					callExpr.Args[0],
				}
			}

		case "fmt.Errorf":
			// Rewrite fmt.Errorf("foo %s", s) to oops.Errorf("foo %s", s).
			callExpr := res.(*ast.CallExpr)
			callExpr.Fun = &ast.SelectorExpr{
				X:   ast.NewIdent("oops"),
				Sel: ast.NewIdent("Errorf"),
			}

			// Cleverly rewrite fmt.Errorf("fail: %s", err) to oops.Wrapf(err, "fail").
			// XXX: this changes behavior is err is nil! oops will ignore it...
			// XXX: how about fmt.Errorf("foo: %s", err.Error())
			if value := o.info.Types[callExpr.Args[0]].Value; value != nil {
				hadSuffix := false
				idx := len(callExpr.Args) - 1
				str := constant.StringVal(value)

				// Check if the format string ends with ": %s" or ": %v"
				for _, suffix := range []string{": %s", ": %v"} {
					if strings.HasSuffix(str, suffix) {
						hadSuffix = true
						str = strings.TrimSuffix(str, suffix)
						break
					}
				}

				// If it did, and the value is an error, rewrite.
				if hadSuffix && o.isErrorExpr(callExpr.Args[idx]) {
					callExpr.Fun = &ast.SelectorExpr{
						X:   ast.NewIdent("oops"),
						Sel: ast.NewIdent("Wrapf"),
					}
					callExpr.Args[0] = &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(str)}
					arg := callExpr.Args[idx]
					copy(callExpr.Args[1:], callExpr.Args)
					callExpr.Args[0] = arg
				}
			}

		default:
			// Replace all other values x with oops.Wrapf(x, "").
			n.Results[i] = &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent("oops"),
					Sel: ast.NewIdent("Wrapf"),
				},
				Args: []ast.Expr{
					n.Results[i],
					&ast.BasicLit{Kind: token.STRING, Value: `""`},
				},
			}
		}
	}

	return n, true
}

// isErrorExpr returns if the expression could be used as an error.
func (o *oopsify) isErrorExpr(n ast.Expr) bool {
	typeAndValue, ok := o.info.Types[n]
	if !ok {
		return false
	}

	return types.Implements(typeAndValue.Type, errorType)
}

// rewriteBinaryExpr rewrites expressions that compare errors to use oops.Cause.
// If the left-hand side of a binary expression is an error, and the right-hand
// side is not nil, we might be comparing an oopsified-error against a constant,
// so unwrap it with oops.Cause.
func (o *oopsifyState) rewriteBinaryExpr(n *ast.BinaryExpr) (ast.Node, bool) {
	// Don't do anything if the left-hand is not an error.
	if !o.isErrorExpr(n.X) {
		return n, true
	}

	// Allow comparisons with nil, since oops.Wrapf(nil, ...) is nil.
	if ident, ok := n.Y.(*ast.Ident); ok && ident.Name == "nil" {
		return n, true
	}

	// If the error is not yet unwrapped, unwrap it.
	if o.funcName(n.X) != "github.com/samsarahq/go/oops.Cause" {
		n.X = &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("oops"),
				Sel: ast.NewIdent("Cause"),
			},
			Args: []ast.Expr{
				n.X,
			},
		}
	}

	return n, true
}

// rewriteCallExpr rewrites calls to strings.Contains(err.Error(), "...") to use
// oops.Cause(err).Error(). Printing the entire stack trace can be pretty
// expensive, and might contain unexpected strings.
func (o *oopsifyState) rewriteCallExpr(n *ast.CallExpr) (ast.Node, bool) {
	// Only rewrite strings.Contains.
	if o.funcName(n) != "strings.Contains" {
		return n, true
	}

	// Check that the first argument is a call to Error().
	callExpr, ok := n.Args[0].(*ast.CallExpr)
	if !ok {
		return n, true
	}
	selectExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return n, true
	}
	if !o.isErrorExpr(selectExpr.X) || selectExpr.Sel.Name != "Error" {
		return n, true
	}

	// If the object is not yet unwrapped, insert a call to oops.Cause.
	if o.funcName(selectExpr.X) != "github.com/samsarahq/go/oops.Cause" {
		selectExpr.X = &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("oops"),
				Sel: ast.NewIdent("Cause"),
			},
			Args: []ast.Expr{
				selectExpr.X,
			},
		}
	}

	return n, true
}

// rewriteTypeAssertExpr rewrites type assertions on errors to call oops.Cause.
// A type assertion might be used to handle certain errors in a special way.
// Errors wrapped with oops.Wrapf will have a different type, and no longer
// match the type check. Try to keep program behavior the same.
func (o *oopsifyState) rewriteTypeAssertExpr(n *ast.TypeAssertExpr) (ast.Node, bool) {
	// Only rewrite type assertions of error objects.
	// XXX: This ignores interfaces that happen to also be errors.
	if !o.isErrorExpr(n.X) {
		return n, true
	}

	// If the error is not yet unwrapped, insert a call to oops.Cause.
	if o.funcName(n.X) != "github.com/samsarahq/go/oops.Cause" {
		n.X = &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent("oops"),
				Sel: ast.NewIdent("Cause"),
			},
			Args: []ast.Expr{
				n.X,
			},
		}
	}

	return n, true
}

// rewrite dispatches to the appropriate rewrite call and maintains local state.
func (o *oopsifyState) rewrite(n ast.Node) (ast.Node, bool) {
	switch n := n.(type) {
	case *ast.ReturnStmt:
		return o.rewriteReturnStmt(n)
	case *ast.BinaryExpr:
		return o.rewriteBinaryExpr(n)
	case *ast.CallExpr:
		return o.rewriteCallExpr(n)
	case *ast.TypeAssertExpr:
		return o.rewriteTypeAssertExpr(n)
	case *ast.FuncLit:
		// Track the current enclosing function.
		o := &oopsifyState{
			oopsify:  o.oopsify,
			function: n.Type,
		}
		n.Body = astrewrite.Walk(n.Body, o.rewrite).(*ast.BlockStmt)
		return n, false
	case *ast.FuncDecl:
		// Track the current enclosing function.
		o := &oopsifyState{
			oopsify:  o.oopsify,
			function: n.Type,
		}
		n.Body = astrewrite.Walk(n.Body, o.rewrite).(*ast.BlockStmt)
		return n, false
	default:
		return n, true
	}
}

// astPrinter formats code like gofmt does.
var astPrinter = printer.Config{
	Mode:     printer.UseSpaces | printer.TabIndent,
	Tabwidth: 8,
}

func main() {
	// XXX: The globbing and package parsing logic belongs in some util package.
	if len(os.Args) < 2 {
		log.Fatal("usage: oopsify <pkg1> <pkg2> ...")
	}
	dirs := os.Args[1:]

	for _, dir := range dirs {
		// Locate the package.
		path := path.Join(os.Getenv("GOPATH"), "src", dir)

		// Initialize state.
		o := &oopsify{
			info: types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
				Uses:  make(map[*ast.Ident]types.Object),
			},
			fset: token.NewFileSet(),
		}

		// Parse the package.
		pkgs, err := parser.ParseDir(o.fset, path, nil, parser.ParseComments|parser.AllErrors)
		if pkgs == nil {
			log.Fatal(err) // parse error
		} else if err != nil {
			// XXX: hack so we can run even if parsing fails. err != nil { for _test.go wonkyness.
			log.Println("ignoring parse error", err)
		}

		conf := types.Config{
			Importer: importer.Default(),
		}

		for _, pkg := range pkgs {
			var files []*ast.File
			for _, file := range pkg.Files {
				files = append(files, file)
			}

			// Type-check the package.
			_, err = conf.Check(dir, o.fset, files, &o.info)
			if err != nil { // XXX: ignore for _test.go wonkiness
				log.Println("ignoring type check error", err)
				// log.Fatal(err) // type error
			}

			// Rewrite each file.
			for filename, file := range pkg.Files {
				// Ignore auto-generated files.
				// XXX: Detect them more elegantly?
				if len(file.Comments) > 0 && len(file.Comments[0].List) > 0 && strings.Contains(file.Comments[0].List[0].Text, "DO NOT EDIT") {
					log.Println("skipping", filename)
					continue
				}

				o := &oopsifyState{
					oopsify: o,
				}
				pkg.Files[filename] = astrewrite.Walk(file, o.rewrite).(*ast.File)

				sourceFile, err := os.OpenFile(filename, os.O_RDWR|os.O_TRUNC, 0664)
				if err != nil {
					log.Fatal(err)
				}
				astPrinter.Fprint(sourceFile, o.fset, file)
				sourceFile.Close()

				if err := exec.Command("goimports", "-w", filename).Run(); err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}
