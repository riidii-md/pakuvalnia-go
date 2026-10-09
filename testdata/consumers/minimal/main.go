package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("fixture " + version)
		return
	}
	fmt.Println("fixture")
}
