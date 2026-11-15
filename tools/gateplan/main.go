package main

import "os"

func main() {
	os.Exit(Command{Output: os.Stdout, Errors: os.Stderr}.Run(os.Args[1:]))
}
