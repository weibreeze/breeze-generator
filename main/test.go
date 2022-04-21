package main

import (
	"fmt"
	"github.com/weibreeze/breeze-generator"
	breezeHttp "github.com/weibreeze/breeze-generator/http"
	"net/http"
	"strconv"
)

func main() {
	testGenerateCode() // generate by local breeze file

	//startGenerateServer(8899, "/generate_code") // as generate server
}

func testGenerateCode() {
	//parsers.UniformPackage = "motan" // set UniformPackage if you want all class in same package.
	//path := "./main"
	path := "./main/demo.breeze"
	config := &generator.Config{WritePath: "./autoGenerate", CodeTemplates: "all", Options: make(map[string]string)}
	//config.Options[templates.GoPackagePrefix] = "myproject/"
	config.Options["with_motan_config"] = "true"
	config.Options["motan_config_type"] = "yaml"
	result, err := generator.GeneratePath(path, config)
	fmt.Printf("%v, %v\n", result, err)
}

func startGenerateServer(port int, path string) {
	http.Handle(path, &breezeHttp.GenerateCodeHandler{})
	http.ListenAndServe(":"+strconv.Itoa(port), nil)
	select {}
}
