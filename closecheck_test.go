package closecheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"testing"

	"golang.org/x/tools/go/loader"
)

func TestCheck(t *testing.T) {
	var conf loader.Config
	conf.ParserMode = parser.ParseComments
	conf.CreateFromFilenames("testdata", "testdata/testdata.go")
	prog, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}

	cmap := ast.NewCommentMap(prog.Fset, prog.Created[0].Files[0], prog.Created[0].Files[0].Comments)

	c := New()
	notClosed := c.Check(prog, prog.Created[0])

	if len(c.objs) == 0 {
		t.Fatal("no objects found")
	}

	for _, obj := range c.objs {
		pos := c.lprog.Fset.Position(obj.node.Pos())

		cmt := cmap[obj.node]
		switch cmt[0].List[0].Text {
		case "// closed":
			// make sure we don't find it in notClosed
			_ = notClosed
			for _, ncObj := range notClosed {
				if ncObj.node == obj.node {
					t.Errorf("%v not closed when should be", pos)
				}
			}
		case "// open":
			seen := false
			for _, ncObj := range notClosed {
				if ncObj.node == obj.node {
					seen = true
				}
			}
			if !seen {
				t.Errorf("%v not in not closed when should be", pos)
			}
		case "// returnArg":
			// find it in returnArgs
			if _, ok := c.returnArgs[obj.id]; !ok {
				t.Errorf("%v not in returnArgs", pos)
			}
		case "// funcArg":
			// find it in funcArgs
			if _, ok := c.funcArgs[obj.id]; !ok {
				t.Errorf("%v not in funcArgs", pos)
			}
		default:
			panic(fmt.Sprintf("%v unknown comment: %q", pos, cmt[0].List[0].Text))
		}
	}
}
