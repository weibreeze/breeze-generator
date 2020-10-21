package main

import (
	"flag"
	"fmt"

	"github.com/weibreeze/breeze-generator/core"

	"github.com/weibreeze/breeze-generator"
)
var (
	srcDir=""
)
func main() {
	flag.StringVar(&srcDir,"src","./main","breeze schema files path")
	flag.Parse()
	testGenerateCode()
}

func testGenerateCode() {
	//parsers.UniformPackage = "motan" // set UniformPackage if you want all class in same package.
	path := srcDir
	//path := "./main/testmsg.breeze"
	config := &generator.Config{WritePath: "./autoGenerate", CodeTemplates: "all", Options: make(map[string]string)}
	//config.Options[templates.GoPackagePrefix] = "myproject/"
	config.Options[core.WithPackageDir] = "true"
	result, err := generator.GeneratePath(path, config)
	fmt.Printf("%v, %v\n", result, err)
}
