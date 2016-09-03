package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bradleyfalzon/closecheck"
	"golang.org/x/tools/go/loader"
)

func main() {
	var conf loader.Config
	if _, err := conf.FromArgs(os.Args[1:], true); err != nil {
		log.Fatalf("Could not check %v: %s\n", os.Args[1:], err)
	}

	prog, err := conf.Load()
	if err != nil {
		log.Fatalf("Could not check %v: %s\n", os.Args[1:], err)
	}

	var ok = true
	for _, pi := range prog.Imported {
		if pi.Errors != nil {
			log.Println("Cannot check package:", pi.Pkg.Name())
			for _, err := range pi.Errors {
				log.Printf("\t%s\n", err)
			}
			os.Exit(1)
		}

		if !pi.TransitivelyErrorFree {
			log.Fatalf("Cannot check package %s: not error free", pi.Pkg.Name())
		}

		notClosed := closecheck.Check(pi, prog.Fset)
		for _, pos := range notClosed {
			ok = false
			// TODO add ident (or line?)
			// TODO add relative path not abs
			fmt.Fprintf(os.Stderr, "%s: is not closed\n", prog.Fset.Position(pos))
		}
	}

	if !ok {
		os.Exit(1)
	}
}
