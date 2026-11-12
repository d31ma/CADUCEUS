package main

import "os"

func main() {
	os.Exit(Command{Output: os.Stdout}.Run(os.Args[1:]))
}
