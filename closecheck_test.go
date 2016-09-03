package closecheck

import (
	"go/token"
	"reflect"
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

	expected := []token.Pos{344, 422}

	positions := Check(prog.Created[0])
	if !reflect.DeepEqual(positions, expected) {
		t.Errorf("got %v unchecked positions, expected %v", positions, expected)
	}
}
