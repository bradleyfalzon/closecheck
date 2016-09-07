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

	for _, obj := range c.objs {

		cmt := cmap[obj.node]
		switch cmt[0].List[0].Text {
		case "// closed":
			// make sure we don't find it in notClosed
			_ = notClosed
			for _, ncObj := range notClosed {
				if ncObj.node == obj.node {
					t.Errorf("not closed when should be")
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
				t.Errorf("not in not closed when should be")
			}
		default:
			// make sure it was returned in notClosed
			switch cmt[0].List[0].Text {
			case "// returnArg":
				// find it in returnArgs
				if _, ok := c.returnArgs[obj.node.Pos()]; !ok {
					t.Errorf("not in returnArgs")
				}
			case "// funcArg":
				// find it in funcArgs
				if _, ok := c.funcArgs[obj.node.Pos()]; !ok {
					t.Errorf("not in funcArgs")
				}
			default:
				panic(fmt.Sprintf("unknown comment: %q", cmt[0].List[0].Text))
			}
		}
	}
}
