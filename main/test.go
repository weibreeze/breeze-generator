package main

import (
	"flag"
	"fmt"
	"github.com/weibreeze/breeze-generator"
	"github.com/weibreeze/breeze-generator/core"
	breezeHttp "github.com/weibreeze/breeze-generator/http"
	"net/http"
	"os"
	"strconv"
)

var (
	srcDir    = ""
	goPkgPath = ""
)

func main() {
	//testGenerateCode() // generate by local breeze file

	startGenerateServer(8899, "/generate_code") // start as generate server
}

func testGenerateCode() {
	os.RemoveAll("autoGenerate")
	defaultPath := "./main/demo.breeze"
	flag.StringVar(&srcDir, "src", defaultPath, "breeze schema files path")
	flag.StringVar(&goPkgPath, "gopkg", "", "project package path in $GOPATH")
	flag.Parse()
	//parsers.UniformPackage = "motan" // set UniformPackage if you want all class in same package.

	path := srcDir
	config := &generator.Config{WritePath: "./autoGenerate", CodeTemplates: "all", Options: make(map[string]string)}
	config.Options[core.WithPackageDir] = "true"
	config.Options[core.GoPackagePrefix] = goPkgPath

	// for test
	//config.Options["with_motan_config"] = "true"
	//config.Options["motan_config_type"] = "yaml"

	result, err := generator.GeneratePath(path, config)
	fmt.Printf("%v, %v\n", result, err)
}

func startGenerateServer(port int, path string) {
	http.Handle(path, &breezeHttp.GenerateCodeHandler{})
	http.ListenAndServe(":"+strconv.Itoa(port), nil)
	select {}
}
