package closecheck

import (
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

	expected := 2

	positions := Check(prog.Created[0])
	if len(positions) != expected {
		t.Errorf("Found %v uncheck positions, expected %d", len(positions), expected)
	}
}
