package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"os"

	"golang.org/x/tools/go/loader"
)

func main() {
	path := "github.com/bradleyfalzon/closecheck/testdata"

	log.Println("Checking:", path)
	ok, err := checkImport(path)
	if err != nil {
		log.Fatalf("Could not check %s: %s", path, err)
	}

	if !ok {
		os.Exit(1)
	}
}

func checkImport(path string) (ok bool, err error) {
	var conf loader.Config

	// Swap to from args, to support ./..., importPath, or . ?
	conf.Import(path)

	// import io to use io.Closer in types.Implements()
	conf.Import("io")

	prog, err := conf.Load()
	if err != nil {
		return false, err
	}

	checker := checker{}
	for ident, def := range prog.Imported["io"].Info.Defs {
		if ident.Name == "Closer" {
			// Surely this can be simplified ?!?!?! - yes see types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
			checker.closer = def.(*types.TypeName).Type().(*types.Named).Underlying().(*types.Interface)
			break
		}
	}
	delete(prog.Imported, "io")

	ok = true
	for _, pi := range prog.Imported {
		notClosed := checker.checkPkgInfo(pi)
		for _, pos := range notClosed {
			ok = false
			// TODO add ident (or line?)
			// TODO add relative path not abs
			fmt.Fprintf(os.Stderr, "%s: is not closed\n", prog.Fset.Position(pos))
		}
	}

	return ok, nil
}

type checker struct {
	closer *types.Interface // stdlib io package for use in types.Implements
}

// checkPkgInfo checks a package info and returns non nil slice of token.Pos if
// any io.Closers are not closed
func (c checker) checkPkgInfo(pi *loader.PackageInfo) []token.Pos {
	if pi.Errors != nil {
		log.Println("Cannot check package:", pi.Pkg.Name())
		for _, err := range pi.Errors {
			log.Printf("\t%s\n", err)
		}
		return nil
	}

	if !pi.TransitivelyErrorFree {
		log.Printf("Cannot check package %s: not error free", pi.Pkg.Name())
		return nil
	}

	v := &visitor{
		pi:     pi,
		closer: c.closer,
		closed: make(map[token.Pos]bool),
	}

	for _, file := range pi.Files {
		for _, decl := range file.Decls {
			ast.Walk(v, decl)
		}
	}
	return v.findNotClosed()
}

type visitor struct {
	closer  *types.Interface
	pi      *loader.PackageInfo
	closers []token.Pos        // closers found
	closed  map[token.Pos]bool // closers closed, just the presence in map is checked
}

func (v *visitor) addCloser(pos token.Pos) {
	v.closers = append(v.closers, pos)
}

func (v *visitor) addClosed(pos token.Pos) {
	v.closed[pos] = true
}

func (v *visitor) findNotClosed() []token.Pos {
	var notClosed []token.Pos
	for _, pos := range v.closers {
		if _, ok := v.closed[pos]; !ok {
			notClosed = append(notClosed, pos)
		}
	}
	return notClosed
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.AssignStmt:
		for _, lhs := range n.Lhs {
			if lhs.(*ast.Ident).Name == "_" {
				continue
			}

			// Get the underlying type of the assigned ident
			lhsType := v.pi.ObjectOf(lhs.(*ast.Ident)).Type()

			if types.Implements(lhsType, v.closer) {
				// lhs implements closer, this will need to be closed
				v.addCloser(lhs.Pos())
			}
		}
	case *ast.CallExpr:
		// closes if a Close method is called
		if fun, ok := n.Fun.(*ast.SelectorExpr); ok {
			if fun.Sel.Name == "Close" {
				// selector is a close, note the ident that's closed
				v.addClosed(v.pi.ObjectOf(fun.X.(*ast.Ident)).Pos())
			}
		}
	case *ast.ExprStmt:
		// closes if it's passed as an argument to a function that accepts an io.Closer
		if fun, ok := n.X.(*ast.CallExpr); ok {
			tuples := v.pi.ObjectOf(fun.Fun.(*ast.Ident)).Type().(*types.Signature).Params()

			// Loop through each function's parameters
			for i := 0; i < tuples.Len(); i++ {
				iface, ok := tuples.At(i).Type().Underlying().(*types.Interface)
				if !ok {
					continue
				}
				if interfaceCloses(iface) {
					// Function's argument requires an io.Closer, it will likely close it
					argIdent := fun.Args[i].(*ast.Ident)
					if argIdent != nil {
						v.addClosed(v.pi.ObjectOf(argIdent).Pos())
					}
				}
			}

		}
	}
	return v
}

// interfaceCloses returns true if an interface has a Close() method or
// embeds an interface that does, returns false otherwise.
func interfaceCloses(iface *types.Interface) bool {
	for i := 0; i < iface.NumMethods(); i++ {
		if iface.Method(i).Name() == "Close" {
			return true
		}
	}
	return false
}
