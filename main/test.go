package main

import (
	"flag"
	"fmt"
	"github.com/weibreeze/breeze-generator"
	"github.com/weibreeze/breeze-generator/core"
	"os"
)
var (
	srcDir=""
	goPkgPath=""
)
func main() {
	os.RemoveAll("autoGenerate")
	flag.StringVar(&srcDir,"src","./main","breeze schema files path")
	flag.StringVar(&goPkgPath,"gopkg","","project package path in $GOPATH")
	flag.Parse()
	testGenerateCode()
}

func testGenerateCode() {
	//parsers.UniformPackage = "motan" // set UniformPackage if you want all class in same package.
	path := srcDir
	config := &generator.Config{WritePath: "./autoGenerate", CodeTemplates: "all", Options: make(map[string]string)}
	config.Options[core.WithPackageDir] = "true"
	config.Options[core.GoPackagePrefix] = goPkgPath
	result, err := generator.GeneratePath(path, config)
	fmt.Printf("%v, %v\n", result, err)
}
