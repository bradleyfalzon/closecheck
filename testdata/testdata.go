package testdata

import (
	"io"
	"os"
)

func testdata1() {
	{
		file, _ := os.Open("/tmp/closecheck")
		_ = file.Close()
	}

	{
		file, _ := os.Open("/tmp/closecheck")
		defer file.Close()
	}

	{
		file, _ := os.Open("/tmp/closecheck")
		closer(file)
	}

	{
		file, _ := os.Open("/tmp/closecheck")
		readCloser(file)
	}

	{
		// Not closed
		file, _ := os.Open("/tmp/closecheck")
		reader(file)
	}

	{
		// Not closed
		file, _ := os.Open("/tmp/closecheck")
		osFileNotClosed(file)
	}

	{
		file, _ := os.Open("/tmp/closecheck")
		osFile(file)
	}

	{
		file, _ := os.Open("/tmp/closecheck")
		go func(f *os.File) {
			f.Close()
		}(file)
	}

	{
		file, _ := os.Open("/tmp/closecheck")
		go func(f *os.File) {
			osFile(f)
		}(file)
	}

	{
		var file *os.File
		file, _ = os.Open("/tmp/closecheck")
		defer file.Close()
	}
}

var _ io.Closer = (*os.File)(nil) // test don't panic

// funcs of various kinds
func closer(_ io.Closer)         {}
func readCloser(_ io.ReadCloser) {}
func reader(_ io.Reader)         {} // does not close
func osFileNotClosed(_ *os.File) {} // does not close
func osFile(f *os.File)          { _ = f.Close() }
