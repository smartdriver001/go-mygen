package main

import "github.com/yezihack/gomygen"

func main() {
	// go-gen
	gomygen.Cmd()
}

/*
example:
gomygen/output/gomygen -h localhost -P 3308 -u root -p 123456 -d kindled
*/
