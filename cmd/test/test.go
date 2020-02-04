package main

import (
	"fmt"
	"os"

	"github.com/weibreeze/breeze-generator/pkg"
	"github.com/weibreeze/breeze-generator/pkg/core"
	"github.com/weibreeze/breeze-generator/pkg/templates"
)

func main() {
	testGenerateCode()
}

func testGenerateCode() {
	//parsers.UniformPackage = "motan" // set UniformPackage if you want all class in same package.
	path := "./cmd/test/testmsg.breeze"
	config := &pkg.Config{WritePath: "./autoGenerate", CodeTemplates: "all", Options: map[string]string{
		core.PackageVersion:              "1.0.0",
		templates.OptionJavaMavenProject: "com.weibo:breeze-demo-api",
	}}
	_ = os.RemoveAll(config.WritePath)
	//config.Options[templates.GoPackagePrefix] = "myproject/"
	result, err := pkg.GeneratePath(path, config)
	fmt.Printf("%v, %v\n", result, err)
}
