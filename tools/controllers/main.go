// Command controllers checks that every exported method of every type in
// apps/<app>/server/controllers/*Controller.go is one statement that
// returns one call on one of the controller's own fields (ADR 0009).
//
//	go run ./tools/controllers apps/<app> [apps/<app> ...]
package main

import "os"

func main() {
	os.Exit(Command{Output: os.Stdout}.Run(os.Args[1:]))
}
