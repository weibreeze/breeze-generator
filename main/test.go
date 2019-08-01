package main

import (
	"fmt"

	"github.com/weibreeze/breeze-generator"
)

func main() {
	testGenerateCode()
}

func testGenerateCode() {
	//parsers.UniformPackage = "motan" // set UniformPackage if you want all class in same package.
	path := "./main"
	//path := "./main/testmsg.breeze"
	config := &generator.Config{WritePath: "./autoGenerate", CodeTemplates: "php, go, java", Options: make(map[string]string)}
	//config.Options[templates.GoPackagePrefix] = "myproject/"
	result, err := generator.GeneratePath(path, config)
	fmt.Printf("%v, %v\n", result, err)
}
