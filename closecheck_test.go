package closecheck

import (
	"go/token"
	"testing"

	"golang.org/x/tools/go/loader"
)

func TestCheck(t *testing.T) {
	var conf loader.Config
	conf.CreateFromFilenames("testdata", "testdata/testdata.go")
	prog, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}

	expected := map[token.Pos]bool{
		345: true, 423: true,
	}

	objs := Check(prog, prog.Created[0])

	for _, obj := range objs {
		if _, ok := expected[obj.Pos()]; !ok {
			t.Errorf("Expected %q (pos %v) to be closed", obj, obj.Pos())
			continue
		}
		delete(expected, obj.Pos())
	}

	for pos := range expected {
		t.Errorf("Expected pos %v to be unclosed", pos)
	}
}
