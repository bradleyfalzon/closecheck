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

// TODO
//func testdata4() (f *os.File) { // returnArg
//f, _ = os.Open("/tmp/closecheck")
//return
//}

type S struct {
	f *os.File
}

func osFile(f *os.File) {}
