package testdata

import "os"

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

type S struct {
	m1 *os.File
	m2 *os.File
}

func osFile(f *os.File) {}
