package main

import "os"

const version = "dev"

func main() { os.Exit(Run(os.Args, os.Stdout, os.Stderr)) }
