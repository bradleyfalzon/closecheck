package main

import (
	"io"
	"os"
)

func main() {

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
		closer(file, nil)
	}

	{
		file, _ := os.Open("/tmp/closecheck")
		closer(nil, file)
	}

}

// closer is an example func that would likely call a close method as it
// accepts an io.Closer
func closer(_ io.Closer, _ io.ReadCloser) {}
