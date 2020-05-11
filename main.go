package main

import "os"

var version = "dev"

func main() { os.Exit(Run(os.Args, os.Stdout, os.Stderr)) }
