package closecheck

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/loader"
)

// ioCloser used to test if a type implements io.Closer using types.Implements()
var ioCloser *types.Interface

func init() {
	var conf loader.Config
	conf.Import("io")
	prog, _ := conf.Load()
	ioCloser = prog.Imported["io"].Pkg.Scope().Lookup("Closer").Type().Underlying().(*types.Interface)
}

// Check an error free loader.PackageInfo and returns non nil slice of
// token.Pos if any io.Closers are not closed.
func Check(pi *loader.PackageInfo) []token.Pos {
	v := &visitor{
		pi:     pi,
		closed: make(map[token.Pos]bool),
	}
	for _, file := range pi.Files {
		for _, decl := range file.Decls {
			ast.Walk(v, decl)
		}
	}
	return v.notClosed()
}

type visitor struct {
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

func (v *visitor) notClosed() []token.Pos {
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

			if types.Implements(lhsType, ioCloser) {
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
