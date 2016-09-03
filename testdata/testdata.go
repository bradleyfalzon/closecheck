package testdata

import (
	"io"
	"os"
)

func testdata() {
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
		osFile(file)
	}
}

// closer is an example func that would likely call a close method as it
// accepts an io.Closer
func closer(_ io.Closer) {}

func readCloser(_ io.ReadCloser) {}

func reader(_ io.Reader) {}

func osFile(_ *os.File) {}
