package closecheck

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
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
	// uses the definition's token.Pos and ignores any selector indexes
	funcArgs map[token.Pos]bool
	// returnArgs contains all ident definition positions that are used as return parameter
	// uses the definition's token.Pos and ignores any selector indexes
	returnArgs map[token.Pos]bool
}

func New() *Checker {
	return &Checker{
		closed:     make(map[string]bool),
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
	id  string    // object's identifier (position + selector index)
	pos token.Pos // object's definition position
	// track where assignment occurred, for display purposes only and the
	// test package uses node for comments
	node ast.Node
}

func (o obj) Pos() token.Pos {
	return o.node.Pos()
}

func (c *Checker) track(pos token.Pos, index []int, node ast.Node) {
	fmt.Printf("%v tracking\n", c.lprog.Fset.Position(pos))
	c.objs = append(c.objs, obj{makeID(pos, index), pos, node})
}

//func (c *Checker) addFuncArg(id string) {
func (c *Checker) addFuncArg(pos token.Pos) {
	fmt.Printf("%v adding func arg\n", c.lprog.Fset.Position(pos))
	c.funcArgs[pos] = true
}

//func (c *Checker) addReturnArg(id string) {
func (c *Checker) addReturnArg(pos token.Pos) {
	fmt.Printf("%v adding return arg\n", c.lprog.Fset.Position(pos))
	c.returnArgs[pos] = true
}

func (c *Checker) addClosed(pos token.Pos, index []int) {
	id := makeID(pos, index)
	fmt.Printf("%v adding closed id %v\n", c.lprog.Fset.Position(pos), id)
	c.closed[id] = true
}

func (c *Checker) notClosed() (objs []obj) {
	fmt.Printf("Checking...\n")
	for _, obj := range c.objs {
		// explicitly closed
		if _, ok := c.closed[obj.id]; ok {
			fmt.Printf("%v closed\n", c.lprog.Fset.Position(obj.node.Pos()))
			continue
		}
		// return argument
		if _, ok := c.returnArgs[obj.pos]; ok {
			fmt.Printf("%v return arguments are ignored\n", c.lprog.Fset.Position(obj.node.Pos()))
			continue
		}
		// function argument
		if _, ok := c.funcArgs[obj.pos]; ok {
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
			pos, index, typ, ok := c.resolveExpr(lhs)
			if !ok {
				continue
			}

			if types.Implements(typ, ioCloser) {
				// lhs implements closer, this will need to be closed
				c.track(pos, index, n)
			}
		}
	case *ast.CallExpr:
		// closes if a Close method is called
		if fun, ok := n.Fun.(*ast.SelectorExpr); ok {
			if fun.Sel.Name == "Close" {
				// selector is a close, note the ident that's closed
				pos, index, _, ok := c.resolveExpr(fun.X)
				if !ok {
					fmt.Printf("%v unsupported type on Close method %T\n", c.lprog.Fset.Position(fun.X.Pos()), fun.X)
					break
				}
				c.addClosed(pos, index)
			}
		}

		// track arguments to func calls, these currently cannot be reliably close checked
		for _, arg := range n.Args {
			pos, _, _, ok := c.resolveExpr(arg)
			if !ok {
				fmt.Printf("%v unsupported type in CallExpr %T\n", c.lprog.Fset.Position(arg.Pos()), arg)
				break
			}
			c.addFuncArg(pos)
		}
	case *ast.FuncDecl:
		// Accepting or returning types defined in function declaration
		for _, arg := range n.Type.Params.List {
			// Exclude function arguments, it maybe closed by the invoker
			for _, ident := range arg.Names {
				pos, _, _, ok := c.resolveExpr(ident)
				if !ok {
					fmt.Printf("%v unsupported type %T\n", c.lprog.Fset.Position(ident.Pos()), ident)
					break
				}
				c.addFuncArg(pos)
			}
		}
		if n.Type.Results != nil {
			// Exclude return arguments, it maybe closed by the invoker
			for _, arg := range n.Type.Results.List {
				for _, ident := range arg.Names {
					pos, _, _, ok := c.resolveExpr(ident)
					if !ok {
						fmt.Printf("%v unsupported type %T\n", c.lprog.Fset.Position(ident.Pos()), ident)
						break
					}
					c.addReturnArg(pos)
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
			pos, _, _, ok := c.resolveExpr(arg)
			if !ok {
				fmt.Printf("%v unsupported type %T", c.lprog.Fset.Position(arg.Pos()), arg)
				break
			}
			c.addReturnArg(pos)
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
	return nil
}

// resolveExpr returns a unique identifier and typ given an ast.Expr for a
// subset of supported types. If the type is unsupported, ok is set to false,
// and no other return values are valid, else ok is set to true.
func (c *Checker) resolveExpr(e ast.Expr) (pos token.Pos, index []int, typ types.Type, ok bool) {
	def := c.exprDef(e)
	if def == nil {
		return pos, index, typ, false
	}

	switch etype := e.(type) {
	case *ast.Ident:
		if etype.Name == "_" {
			return pos, index, typ, false
		}
		typ = def.Type()
	case *ast.SelectorExpr:
		if _, ok := c.pi.Selections[etype]; !ok {
			return pos, index, typ, false
		}
		typ = c.pi.Selections[etype].Obj().Type()
		index = c.pi.Selections[etype].Index()
	default:
		// Unsupported type
		return pos, index, typ, false
	}

	return def.Pos(), index, typ, true
}

func makeID(pos token.Pos, index []int) (id string) {
	id = strconv.FormatInt(int64(pos), 10)
	for _, idx := range index {
		id += "," + strconv.FormatInt(int64(idx), 10)
	}
	return id
}
