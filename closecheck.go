package closecheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"
	"strconv"

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
	objs   []obj           // objects that need to be checked for closing
	closed map[string]bool // closers closed, just the presence in map is checked
	// funcArgs contains all ident definition positions that are used in function args
	funcArgs map[string]bool
	// returnArgs contains all ident definition positions that are used as return parameter
	returnArgs map[string]bool
}

func New() *Checker {
	return &Checker{
		closed:     make(map[string]bool),
		funcArgs:   make(map[string]bool),
		returnArgs: make(map[string]bool),
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
	id string
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

func (c *Checker) addFuncArg(id string) {
	c.funcArgs[id] = true
}

func (c *Checker) addReturnArg(id string) {
	c.returnArgs[id] = true
}

func (c *Checker) addClosed(id string) {
	c.closed[id] = true
}

func (c *Checker) notClosed() (objs []obj) {
	for _, obj := range c.objs {
		// explicitly closed
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
			id, typ, ok := c.resolveExpr(lhs)
			if !ok {
				continue
			}

			if types.Implements(typ, ioCloser) {
				// lhs implements closer, this will need to be closed
				c.addObj(obj{id: id, node: n})
			}
		}
	case *ast.CallExpr:
		// closes if a Close method is called
		if fun, ok := n.Fun.(*ast.SelectorExpr); ok {
			if fun.Sel.Name == "Close" {
				// selector is a close, note the ident that's closed
				id, _, ok := c.resolveExpr(fun.X)
				if !ok {
					log.Printf("Unsupported type %T", fun.X)
					break
				}
				c.addClosed(id)
			}
		}

		// track arguments to func calls, these currently cannot be reliably close checked
		for _, arg := range n.Args {
			id, _, ok := c.resolveExpr(arg)
			if !ok {
				log.Printf("Unsupported type %T", arg)
				break
			}
			c.addFuncArg(id)
		}
	case *ast.FuncDecl:
		// Accepting or returning types defined in function declaration
		for _, arg := range n.Type.Params.List {
			// Exclude function arguments, it maybe closed by the invoker
			for _, ident := range arg.Names {
				id, _, ok := c.resolveExpr(ident)
				if !ok {
					log.Printf("Unsupported type %T", ident)
					break
				}
				c.addFuncArg(id)
			}
		}
		if n.Type.Results != nil {
			// Exclude return arguments, it maybe closed by the invoker
			for _, arg := range n.Type.Results.List {
				for _, ident := range arg.Names {
					id, _, ok := c.resolveExpr(ident)
					if !ok {
						log.Printf("Unsupported type %T", ident)
						break
					}
					c.addReturnArg(id)
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
			id, _, ok := c.resolveExpr(arg)
			if !ok {
				log.Printf("Unsupported type %T", arg)
				break
			}
			c.addReturnArg(id)
		}
	}
	return c
}

// exprObj returns the result of ObectOf for an expression, if none found
// it will be nil.
func (c *Checker) exprDef(e ast.Expr) types.Object {
	switch f := e.(type) {
	case *ast.SelectorExpr:
		// Resolve the top most ident, eg "a" given:
		// a.b.c.d.e.f.Close()
		// ^
		return c.exprDef(f.X)
	case *ast.Ident:
		return c.pi.ObjectOf(f)
	}
	panic(fmt.Sprintf("unexpected type %T at %s", e, c.lprog.Fset.Position(e.Pos())))
}

// resolveExpr returns a unique identifier and typ given an ast.Expr for a
// subset of supported types. If the type is unsupported, ok is set to false,
// else it's set to true.
func (c *Checker) resolveExpr(e ast.Expr) (id string, typ types.Type, ok bool) {
	var (
		index []int
		def   types.Object
	)
	switch etype := e.(type) {
	case *ast.Ident:
		if etype.Name == "_" {
			return id, typ, false
		}
		def = c.exprDef(e)
		typ = def.Type()
	case *ast.SelectorExpr:
		def = c.exprDef(e)
		typ = c.pi.Selections[etype].Obj().Type()
		index = c.pi.Selections[etype].Index()
	default:
		// Unsupported type
		return id, typ, false
	}

	return makeID(def.Pos(), index), typ, true
}

func makeID(pos token.Pos, index []int) (id string) {
	id = strconv.FormatInt(int64(pos), 10)
	for _, idx := range index {
		id += "," + strconv.FormatInt(int64(idx), 10)
	}
	return id
}
