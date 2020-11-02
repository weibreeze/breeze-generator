package main

import (
	"fmt"
	generator "github.com/weibreeze/breeze-generator"
	"github.com/weibreeze/breeze-generator/core"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
)

func main() {
	defer func() {
		if e:=recover();e!=nil{
			fmt.Println(e)
		}
	}()
	app := kingpin.New("breeze-generator", "toolchain for breeze, https://github.com/weibreeze/breeze/")

	genCMD := app.Command("gen", "")
	gen_typ := genCMD.Flag("type", "generator code type: go, php, java, cpp").Default("all").String()
	gen_src := genCMD.Flag("src", "source path of files .breeze").Default("").String()
	gen_dest := genCMD.Flag("dest", "destination path of generated files").Default("autoGenerate").String()
	gen_go_pkg := genCMD.Flag("gopkg", "prefix of go import package").Default("").String()

	p2bCMD := app.Command("p2b", "convert files .proto to .breeze, rule details: https://github.com/weibreeze/breeze/")
	p2b_src := p2bCMD.Flag("src", "source path of files .proto").Default("").String()
	p2b_dest := p2bCMD.Flag("dest", "destination path of files .breeze").Default("").String()
	if len(os.Args) == 1 {
		app.Usage([]string{"--help"})
		return
	}
	command, err := app.Parse(os.Args[1:])
	if err != nil || len(os.Args) == 1 {
		fmt.Println(err)
		app.Usage([]string{"--help"})
		return
	}
	switch command {
	case "gen":
		if *p2b_src == "" {
			return
		}
		config := &generator.Config{WritePath: *gen_dest, CodeTemplates: *gen_typ, Options: make(map[string]string)}
		config.Options[core.WithPackageDir] = "true"
		if *gen_go_pkg != "" {
			config.Options[core.GoPackagePrefix] = *gen_go_pkg
		}
		_, err = generator.GeneratePath(*gen_src, config)
		if err != nil {
			fmt.Printf("generator fail, error: %s\n", err)
		}
	case "p2b":
		if *p2b_src == "" || *p2b_dest == "" {
			return
		}
		err = generator.ProtoToBreeze(*p2b_src, *p2b_dest)
		if err != nil {
			fmt.Printf("convert fail, error: %s\n", err)
		}
	}
}
