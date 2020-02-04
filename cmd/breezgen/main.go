package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/weibreeze/breeze-generator/pkg"
)

func main() {
	output := flag.String("output", "./auto_gen", "output directory")
	input := flag.String("input", "", "input breeze file or directory")
	languages := flag.String("languages", "all", "languages to generate")
	options := flag.String("options", "", "generate options, mapStringString, eg: option1=a,option2=b")
	flag.Parse()
	if *input == "" {
		fmt.Println("no input specified")
		os.Exit(1)
	}
	generateOptions, err := parseGenerateOptions(*options)
	if err != nil {
		fmt.Println(err.Error())
	}
	_ = os.RemoveAll(*output)
	files, err := pkg.GeneratePath(*input, &pkg.Config{
		WritePath:     *output,
		CodeTemplates: *languages,
		Options:       generateOptions,
	})
	if err != nil {
		fmt.Printf("generate failed: %s", err.Error())
		os.Exit(1)
	}
	for _, f := range files {
		fmt.Println("generated schema: " + f)
	}
}

func parseGenerateOptions(options string) (map[string]string, error) {
	optionsMap := make(map[string]string)
	entries := strings.Split(options, ",")
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		keyAndValue := strings.Split(entry, "=")
		if len(keyAndValue) != 2 {
			return optionsMap, fmt.Errorf("bad option: %s", entry)
		}
		optionsMap[keyAndValue[0]] = keyAndValue[1]
	}
	return optionsMap, nil
}
