package main

import (
	"flag"
	"fmt"
	"go/types"
	"os"

	"github.com/bradleyfalzon/closecheck"
	"github.com/kisielk/gotool"
	"golang.org/x/tools/go/loader"
)

func main() {

	hideErr := flag.Bool("hide-errors", false, "Skip and hide any parsing errors encountered when checking package")
	verbose := flag.Bool("v", false, "Show verbose debugging messages")
	flag.Parse()

	// Use gotool to default blank import path to "." and handle recursion
	paths := gotool.ImportPaths(flag.Args())

	var conf loader.Config
	if _, err := conf.FromArgs(paths, true); err != nil {
		fmt.Fprintf(os.Stderr, "Could not check %v: %s\n", os.Args[1:], err)
		os.Exit(1)
	}

	conf.TypeChecker = types.Config{
		Error: func(err error) {
			if !*hideErr {
				fmt.Fprintf(os.Stderr, "%s\n", err)
			}
		},
	}

	prog, err := conf.Load()
	if err != nil {
		if *hideErr {
			return
		}
		fmt.Fprintf(os.Stderr, "Could not check %v: %s\n", os.Args[1:], err)
		os.Exit(1)
	}

	var ok = true
	for _, pi := range prog.Imported {
		if pi.Errors != nil {
			if *hideErr {
				continue
			}
			fmt.Fprintf(os.Stderr, "Cannot check package: %s\n", pi.Pkg.Name())
			for _, err := range pi.Errors {
				fmt.Fprintf(os.Stderr, "\t%s\n", err)
			}
			os.Exit(1)
		}

		if !pi.TransitivelyErrorFree {
			if *hideErr {
				continue
			}
			fmt.Fprintf(os.Stderr, "Cannot check package %s: not error free\n", pi.Pkg.Name())
			os.Exit(1)
		}

		c := closecheck.New()
		c.Verbose = *verbose
		notClosed := c.Check(prog, pi)
		for _, obj := range notClosed {
			ok = false
			// TODO add relative path not abs
			fmt.Fprintf(os.Stderr, "%s: is not closed\n", prog.Fset.Position(obj.Pos()))
		}
	}

	if !ok {
		os.Exit(2)
	}
}
