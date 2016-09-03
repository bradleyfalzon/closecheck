package closecheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
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
func Check(lprog *loader.Program, pi *loader.PackageInfo) []obj {
	v := &visitor{
		lprog:  lprog,
		pi:     pi,
		closed: make(map[token.Pos]bool),
	}

	for _, file := range pi.Files {
		for _, decl := range file.Decls {
			ast.Walk(v, decl)
		}
	}

	if len(v.notClosed()) == 0 {
		return v.notClosed()
	}

	// Some variables weren't closed, lets check the callstack, perhaps
	// they passed into a function

	prog := ssautil.CreateProgram(v.lprog, 0)

	prog.Build()

	// Using cha as it supports applications without main or test
	// if there's _edge_ cases in the graph algorithm (see what i did there)
	// we could switch to rta and require main/tests
	cg := cha.CallGraph(prog)
	cg.DeleteSyntheticNodes()

	// Given a list of source positions, find which functions they're passed into
	// which will form the start node for the callgraph search

	// Walk the ast again, finding all calls to functions, check to see if the
	// arguments to those functions are the unclosed object's we're still tracking.
	// If so, note the object's position and the function definition's position.

	var poses [][2]token.Pos
	for _, file := range pi.Files {
		for _, decl := range file.Decls {
			ast.Inspect(decl, func(node ast.Node) bool {
				ce, ok := node.(*ast.CallExpr)
				if !ok {
					// We're looking for function calls only
					return true
				}

				var funcPos token.Pos
				switch ce.Fun.(type) {
				case *ast.Ident, *ast.SelectorExpr:
					funcPos = v.pi.ObjectOf(callerIdent(ce.Fun)).Pos()
				case *ast.FuncLit:
					funcPos = ce.Fun.(*ast.FuncLit).Type.Func
				default:
					// Non function, likely ok to ignore it
					return true
				}

				for _, arg := range ce.Args {
					if _, ok := arg.(*ast.Ident); !ok {
						continue
					}
					argObj := v.pi.ObjectOf(arg.(*ast.Ident))
					// Check if one of the function's arguments is one of the unclosed
					// objects we're still trying check
					for _, obj := range v.notClosed() {
						if argObj == obj.types {
							poses = append(poses, [2]token.Pos{obj.types.Pos(), funcPos})
						}
					}
				}
				return true
			})
		}
	}

	// Given an object's position, a function's definition position, and an array
	// of closed object positions, assume that the source object may have been the
	// object being closed in the target function. This isn't precise, and would
	// assume something is closed if any of the closed positions were for
	// different objects.
	//
	// A better way maybe to track a graph/set of idents, determining their types
	// def and following them through the program.

	pkg := prog.Package(v.pi.Pkg)
	if pkg == nil {
		panic(fmt.Errorf("no SSA package"))
	}

	for _, sourcePos := range poses {
		_, sourcePath, _ := v.lprog.PathEnclosingInterval(sourcePos[1], sourcePos[1])
		if !ssa.HasEnclosingFunction(pkg, sourcePath) {
			// this position is not inside a function
			continue
		}

		source := ssa.EnclosingFunction(pkg, sourcePath)
		if source == nil {
			panic(fmt.Errorf("no SSA function built for this location (dead code?)"))
		}

		for targetPos := range v.closed {
			_, targetPath, _ := v.lprog.PathEnclosingInterval(targetPos, targetPos)
			if !ssa.HasEnclosingFunction(pkg, targetPath) {
				panic(fmt.Errorf("this position is not inside a function"))
			}

			target := ssa.EnclosingFunction(pkg, targetPath)
			if target == nil {
				panic(fmt.Errorf("no SSA function built for this location (dead code?)"))
			}

			isEnd := func(n *callgraph.Node) bool { return n.Func == target }

			if cp := callgraph.PathSearch(cg.CreateNode(source), isEnd); cp != nil {
				// we have a path to a closer
				v.addClosed(sourcePos[0])
			}
		}
	}

	return v.notClosed()
}

type visitor struct {
	lprog  *loader.Program
	pi     *loader.PackageInfo
	objs   []obj              // objects that need to be checked for closing
	closed map[token.Pos]bool // closers closed, just the presence in map is checked
}

func (v *visitor) walk(pi *loader.PackageInfo) {
}

type obj struct {
	types     types.Object
	assignPos token.Pos
}

func (o obj) Pos() token.Pos {
	return o.assignPos
}

func (v *visitor) addObj(obj obj) {
	v.objs = append(v.objs, obj)
}

func (v *visitor) addClosed(pos token.Pos) {
	v.closed[pos] = true
}

func (v *visitor) notClosed() (objs []obj) {
	for _, obj := range v.objs {
		if _, ok := v.closed[obj.types.Pos()]; !ok {
			objs = append(objs, obj)
		}
	}
	return objs
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.AssignStmt:
		for _, lhs := range n.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "_" {
				continue
			}

			// Get the underlying type of the assigned ident
			def := v.exprDef(lhs)
			if def == nil {
				// def maybe nil in switch e := T.(type)
				break
			}
			if types.Implements(def.Type(), ioCloser) {
				// lhs implements closer, this will need to be closed
				v.addObj(obj{types: def, assignPos: lhs.Pos()})
			}
		}
	case *ast.CallExpr:
		// closes if a Close method is called
		if fun, ok := n.Fun.(*ast.SelectorExpr); ok {
			if fun.Sel.Name == "Close" {
				// selector is a close, note the ident that's closed

				// Anything defined at def.Pos() is closed
				def := v.exprDef(fun.X)
				v.addClosed(def.Pos())
			}
		}
	case *ast.ExprStmt:
		// closes if it's passed as an argument to a function that accepts an io.Closer
		if fun, ok := n.X.(*ast.CallExpr); ok {
			sig, ok := v.exprDef(fun.Fun).Type().(*types.Signature)
			if !ok {
				break
			}

			// Loop through each function's parameters
			for i := 0; i < sig.Params().Len(); i++ {
				iface, ok := sig.Params().At(i).Type().Underlying().(*types.Interface)
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

// exprObj returns the result of ObectOf for an expression, if none found
// it will be nil.
func (v *visitor) exprDef(e ast.Expr) types.Object {
	switch f := e.(type) {
	case *ast.StarExpr:
		return v.exprDef(f.X)
	case *ast.SelectorExpr:
		return v.pi.ObjectOf(f.Sel)
	case *ast.IndexExpr:
		return v.exprDef(f.X)
	case *ast.CallExpr:
		return v.exprDef(f.Fun)
	case *ast.Ident:
		return v.pi.ObjectOf(f)
	default:
		panic(fmt.Sprintf("unexpected type %T", e))
	}
}

// Given a ast.CallExpr find the ident of the function being called
func callerIdent(e ast.Expr) *ast.Ident {
	switch f := e.(type) {
	case *ast.SelectorExpr:
		return callerIdent(f.Sel)
	case *ast.Ident:
		return f
	}
	panic(fmt.Sprintf("unexpected type %T", e))
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
