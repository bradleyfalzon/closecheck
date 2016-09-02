package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"golang.org/x/tools/go/loader"
)

func main() {

	path := "github.com/bradleyfalzon/closecheck/testdata"

	log.Println("Checking:", path)
	err := checkImport(path)
	if err != nil {
		log.Fatalf("Could not check %s: %s", path, err)
	}

	log.Println("All done")
}

func checkImport(path string) error {

	var conf loader.Config

	// Swap to from args, to support ./..., importPath, or . ?
	conf.Import(path)

	// import io to use io.Closer in types.Implements()
	conf.Import("io")

	prog, err := conf.Load()
	if err != nil {
		return err
	}

	checker := checker{}
	for ident, def := range prog.Imported["io"].Info.Defs {
		if ident.Name == "Closer" {
			// Surely this can be simplified ?!?!?!
			checker.closer = def.(*types.TypeName).Type().(*types.Named).Underlying().(*types.Interface)
			break
		}
	}
	delete(prog.Imported, "io")

	log.Printf("Created len %d", len(prog.Created))
	log.Printf("Imported len %d", len(prog.Imported))
	for name, pi := range prog.Imported {
		log.Printf("checking %s: ", name)
		notClosed := checker.checkPkgInfo(pi)
		log.Printf("%d not closed", len(notClosed))
	}
	log.Printf("All Packages len %d", len(prog.AllPackages))
	//for name := range prog.AllPackages {
	//log.Printf("\t%s", name)
	//}

	return nil
}

type checker struct {
	closer *types.Interface // stdlib io package for use in types.Implements
}

// checkPkgInfo checks a package info and returns non nil slice of token.Pos if
// any io.Closers are not closed
func (c checker) checkPkgInfo(pi *loader.PackageInfo) []token.Pos {
	log.Println("checkPkgInfo")
	if pi.Errors != nil {
		log.Println("Cannot check package:")
		for _, err := range pi.Errors {
			log.Printf("\t%s\n", err)
		}
		return nil
	}

	if !pi.TransitivelyErrorFree {
		log.Println("Cannot check package: not error free")
		return nil
	}
	log.Println("no errors")

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
	log.Println("addCloser:", pos)
	v.closers = append(v.closers, pos)
}

func (v *visitor) addClosed(pos token.Pos) {
	log.Println("addClosed:", pos)
	v.closed[pos] = true
}

func (v *visitor) findNotClosed() []token.Pos {
	var notClosed []token.Pos
	for _, pos := range v.closers {
		log.Println("checking pos:", pos)
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
		if fun, ok := n.Fun.(*ast.SelectorExpr); ok {
			if fun.Sel.Name == "Close" {
				// selector is a close, note the ident that's closed
				v.addClosed(v.pi.ObjectOf(fun.X.(*ast.Ident)).Pos())
			}
		}
	}
	return v
}
