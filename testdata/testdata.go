package testdata

import (
	"fmt"
	"os"
)

func testdata1() {
	{
		f, _ := os.Open("/tmp/closecheck") // closed
		_ = f.Close()
	}

	{
		f, _ := os.Open("/tmp/closecheck") // closed
		defer f.Close()
	}

	{
		f, _ := os.Open("/tmp/closecheck") // closed
		go func() {
			f.Close()
		}()
	}

	{
		f, _ := os.Open("/tmp/closecheck") // funcArg
		osFile(f)
	}

	{
		f, _ := os.Open("/tmp/closecheck") // open
		_ = f
	}

	{
		s := S{}
		s.m1, _ = os.Open("/tmp/closecheck") // open
	}

	{
		s := S{}
		s.m1, _ = os.Open("/tmp/closecheck") // closed
		s.m1.Close()
	}

	{
		s := S{}
		s.m1, _ = os.Open("/tmp/closecheck") // closed
		s.m2, _ = os.Open("/tmp/closecheck") // open
		s.m1.Close()
	}

	{
		s := S{}
		s.m1, _ = os.Open("/tmp/closecheck") // closed
		s.m2, _ = os.Open("/tmp/closecheck") // open
		s.m2, _ = os.Open("/tmp/closecheck") // open
		s.m1.Close()

		s1 := S{}
		s1.m1, _ = os.Open("/tmp/closecheck") // open
		s1.m2, _ = os.Open("/tmp/closecheck") // closed
		s1.m2.Close()
	}

	{
		s := S{}
		s.m1, _ = os.Open("/tmp/closecheck") // funcArg
		osFile(s.m1)
	}

	{
		s := S{}
		s.m1, _ = os.Open("/tmp/closecheck") // funcArg
		osFile2(s)
	}

	return // test handling return with no argument
}

func testdata2() *os.File {
	f, _ := os.Open("/tmp/closecheck") // returnArg
	return f
}

func testdata3() string {
	f, _ := os.Open("/tmp/closecheck") // open
	_ = f
	return "" // test handling non ident argument
}

func testdata4() (f *os.File) {
	f, _ = os.Open("/tmp/closecheck") // returnArg
	return                            // naked return
}

func testdata5(f *os.File) {
	f, _ = os.Open("/tmp/closecheck") // funcArg
}

func testdata6() S {
	s := S{}
	s.m1, _ = os.Open("/tmp/closecheck") // returnArg
	return s
}

func testdata7() *os.File {
	s := S{}
	s.m1, _ = os.Open("/tmp/closecheck") // returnArg
	return s.m1
}

func testdata8() {
	fmt.Fprint(os.Stdout, "") // handle panic
	s := S{}
	s.m3[0].m1, _ = os.Open("/tmp/closecheck") // handle panic on selectorExpr.X.(*ast.IndexExpr)
}

func testdata9() {
	fmt.Fprint(os.Stdout, "")
	f, _ := os.Open("/tmp/closecheck") // funcArg
	s := S{
		m1: f,
	}
	osFile2(s)
}

type S struct {
	m1 *os.File
	m2 *os.File
	m3 []S
}

func osFile(f *os.File) {}
func osFile2(f S)       {}
