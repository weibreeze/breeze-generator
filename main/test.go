package main

import (
	"fmt"

	"github.com/weibreeze/breeze-generator"
)

func main() {
	testGenerateCode()
}

func testGenerateCode() {
	path := "./main"
	config := &generator.Config{WritePath: "./autoGenerate"}
	result, err := generator.GeneratePath(path, config)
	fmt.Printf("%v, %v\n", result, err)
}
