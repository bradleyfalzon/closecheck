package closecheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"

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

type Checker struct {
	lprog  *loader.Program
	pi     *loader.PackageInfo
	objs   []obj              // objects that need to be checked for closing
	closed map[token.Pos]bool // closers closed, just the presence in map is checked
	// funcArgs contains all ident definition positions that are used in function args
	funcArgs map[token.Pos]bool
	// returnArgs contains all ident definition positions that are used as return parameter
	returnArgs map[token.Pos]bool
}

func New() *Checker {
	return &Checker{
		closed:     make(map[token.Pos]bool),
		funcArgs:   make(map[token.Pos]bool),
		returnArgs: make(map[token.Pos]bool),
	}
}

// Check an error free loader.PackageInfo and returns non nil slice of
// token.Pos if any io.Closers are not closed.
func (c *Checker) Check(lprog *loader.Program, pi *loader.PackageInfo) []obj {
	c.lprog = lprog
	c.pi = pi
	for _, file := range pi.Files {
		for _, decl := range file.Decls {
			ast.Walk(c, decl)
		}
	}

	return c.notClosed()
}

type obj struct {
	id token.Pos
	//types types.Type
	// track where assignment occurred, for display purposes only and the
	// test package uses node for comments
	node ast.Node
}

func (o obj) Pos() token.Pos {
	return o.node.Pos()
}

func (c *Checker) addObj(obj obj) {
	log.Printf("adding object: %v", obj.id)
	c.objs = append(c.objs, obj)
}

func (c *Checker) addFuncArg(id token.Pos) {
	c.funcArgs[id] = true
}

func (c *Checker) addReturnArg(id token.Pos) {
	c.returnArgs[id] = true
}

func (c *Checker) addClosed(id token.Pos) {
	c.closed[id] = true
}

func (c *Checker) notClosed() (objs []obj) {
	for _, obj := range c.objs {
		// explicitly closed
		// TODO do I need types anymore? maybe just pos?
		if _, ok := c.closed[obj.id]; ok {
			fmt.Printf("%v closed\n", c.lprog.Fset.Position(obj.node.Pos()))
			continue
		}
		// return argument
		if _, ok := c.returnArgs[obj.id]; ok {
			fmt.Printf("%v return arguments are ignored\n", c.lprog.Fset.Position(obj.node.Pos()))
			continue
		}
		// function argument
		if _, ok := c.funcArgs[obj.id]; ok {
			fmt.Printf("%v function arguments are ignored\n", c.lprog.Fset.Position(obj.node.Pos()))
			continue
		}
		// not closed
		objs = append(objs, obj)
	}
	return objs
}

func (c *Checker) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.AssignStmt:
		for _, lhs := range n.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "_" {
				continue
			}

			// Get the underlying type of the assigned ident

			def := c.exprDef(lhs)

			//spew.Dump(def)
			if types.Implements(def.Type(), ioCloser) {
				// lhs implements closer, this will need to be closed
				c.addObj(obj{id: def.Pos(), node: n})
			}
		}
	case *ast.CallExpr:
		// closes if a Close method is called
		if fun, ok := n.Fun.(*ast.SelectorExpr); ok {
			if fun.Sel.Name == "Close" {
				// selector is a close, note the ident that's closed

				if _, ok := fun.X.(*ast.Ident); !ok {
					// struct.Member.Close(), we don't handle tracking which members need
					// to be closed
					// Maybe we could add to these supported types, such as map, and why can't
					// we use selector, if we're getting the correct memory location (which I
					// think is the issue).
					//log.Printf("Unsupported type %T", fun.X)
					//break
				}

				// Anything defined at def.Pos() is closed
				c.addClosed(c.exprDef(fun.X).Pos())
				//spew.Dump("closer:" + string(idOf(c.exprDef(fun.X))))
				//c.addClosed(idOf(c.exprDef(fun.X)))
			}
		}

		// track arguments to func calls, these currently cannot be reliably close checked

		for _, arg := range n.Args {
			switch arg.(type) {
			case *ast.Ident:
			default:
				// skip *ast.BasicLit etc
				continue
			}
			c.addFuncArg(c.exprDef(arg).Pos())
		}

	case *ast.FuncDecl:
		// Accepting or returning types defined in function declaration
		for _, arg := range n.Type.Params.List {
			// Exclude function arguments, it maybe closed by the invoker
			for _, ident := range arg.Names {
				_ = ident
				c.addFuncArg(c.exprDef(ident).Pos())
			}
		}
		if n.Type.Results != nil {
			// Exclude return arguments, it maybe closed by the invoker
			for _, arg := range n.Type.Results.List {
				for _, ident := range arg.Names {
					_ = ident
					c.addReturnArg(c.exprDef(ident).Pos())
				}
			}
		}
	case *ast.ReturnStmt:
		// Returning a type not already defined in function declaration
		if n.Results == nil {
			// Could be naked return
			break
		}
		for _, arg := range n.Results {
			switch arg.(type) {
			case *ast.Ident:
			default:
				// skip *ast.BasicLit etc
				continue
			}
			c.addReturnArg(c.exprDef(arg).Pos())
		}
	}
	return c
}

// exprObj returns the result of ObectOf for an expression, if none found
// it will be nil.
func (c *Checker) exprDef(e ast.Expr) types.Object {
	switch f := e.(type) {
	case *ast.Ident:
		return c.pi.ObjectOf(f)
	}
	panic(fmt.Sprintf("unexpected type %T at %s", e, c.lprog.Fset.Position(e.Pos())))
}
