package generator

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"github.com/weibreeze/breeze-generator/core"
	"github.com/weibreeze/breeze-generator/parsers"
	"github.com/weibreeze/breeze-generator/templates"
)

//Config is a generate config struct
type Config struct {
	Parser        string
	CodeTemplates string
	WritePath     string
	Options       map[string]string
}

//RegisterParser can register a custom Parser for extension
func RegisterParser(parser core.Parser) {
	parsers.Register(parser)
}

//RegisterCodeTemplate can register a custom CodeTemplate for extension
func RegisterCodeTemplate(template core.CodeTemplate) {
	templates.Register(template)
}

//GeneratePath find all schema files in path, and generate code according config
func GeneratePath(path string, config *Config) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	_, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if config == nil {
		config = &Config{}
	}
	if config.WritePath == "" {
		config.WritePath = path
	}
	context, err := initContext(config)
	if err != nil {
		return nil, err
	}
	err = parseSchemaWithPath(path, context)
	if err != nil {
		return nil, err
	}
	err = generateCode(context)
	if err != nil {
		return nil, err
	}

	fileNames := make([]string, 0, len(context.Schemas))
	for key := range context.Schemas {
		fileNames = append(fileNames, key)
	}
	return fileNames, nil
}

//Generate generate code from binary content
func Generate(name string, content []byte, config *Config) error {
	context, err := initContext(config)
	if err != nil {
		return err
	}
	err = parseSchema(name, content, context)
	if err != nil {
		return err
	}
	return generateCode(context)
}

func parseSchemaWithPath(path string, context *core.Context) error {
	f, _ := os.Open(path)
	fi, err := f.Stat()
	if err != nil {
		return err
	}

	if fi.IsDir() {
		var fileInfo []os.FileInfo
		fileInfo, err = ioutil.ReadDir(fi.Name())
		if err == nil {
			path = addSeparator(path)
			for _, info := range fileInfo {
				subName := path + info.Name()
				errForLog := parseSchemaWithPath(subName, context)
				if errForLog != nil {
					fmt.Printf("warning: process file fail: %s, err:%s\n", subName, errForLog)
					continue
				}
			}
		}
	} else if strings.HasSuffix(fi.Name(), context.Parser.FileSuffix()) {
		var content []byte
		content, err = ioutil.ReadFile(path)
		if err == nil {
			err = parseSchema(fi.Name(), content, context)
		}
	}
	return err
}

func parseSchema(name string, content []byte, context *core.Context) error {
	schema, err := context.Parser.ParseSchema(content, context)
	if err != nil {
		return err
	}
	schema.Name = name
	err = core.Validate(schema)
	if err != nil {
		return err
	}
	//add schemas and messages to context
	context.Schemas[schema.Name] = schema
	for key, value := range schema.Messages {
		context.Messages[schema.Package+"."+key] = value
		for opKey, opValue := range schema.Options {
			if _, ok := value.Options[opKey]; !ok {
				value.Options[opKey] = opValue
			}
		}
	}
	return nil
}

func generateCode(context *core.Context) error {
	oldMask := syscall.Umask(0)
	defer syscall.Umask(oldMask)
	for _, schema := range context.Schemas {
		for _, template := range context.Templates {
			files, err := template.GenerateCode(schema, context)
			if err != nil {
				fmt.Printf("error: generate code fail, template:%s, err:%s\n", template.Name(), err.Error())
				continue
			}
			path := context.WritePath
			if path[len(path)-1:] != string(os.PathSeparator) {
				path += string(os.PathSeparator)
			}
			path = path + template.Name() + string(os.PathSeparator)
			err = os.MkdirAll(path, 0777)
			if err != nil {
				return err
			}
			for name, content := range files {
				index := strings.LastIndex(name, string(os.PathSeparator)) //contains path
				if index > -1 {
					err := os.MkdirAll(path+name[:index+1], 0777)
					if err != nil {
						return err
					}
				}
				err = ioutil.WriteFile(path+name, content, 0666)
				if err != nil {
					fmt.Printf("error: write code fail, template:%s, file name:%s, err:%s\n", template.Name(), name, err.Error())
				}
			}
		}
	}
	return nil
}

func initContext(config *Config) (*core.Context, error) {
	if config == nil {
		config = &Config{}
	}
	if config.Parser == "" {
		config.Parser = parsers.Breeze
	}
	if config.CodeTemplates == "" {
		config.CodeTemplates = templates.All
	}
	if config.WritePath == "" {
		config.WritePath = "./"
	}
	config.WritePath = addSeparator(config.WritePath)
	context := &core.Context{Parser: parsers.GetParser(config.Parser), Schemas: make(map[string]*core.Schema), Messages: make(map[string]*core.Message), WritePath: config.WritePath}
	if config.Options != nil {
		context.Options = config.Options
	} else {
		context.Options = make(map[string]string)
	}
	if context.Parser == nil {
		return nil, errors.New("can not find parser: " + config.Parser)
	}
	var err error
	context.Templates, err = templates.GetTemplate(config.CodeTemplates)
	return context, err
}

func addSeparator(path string) string {
	if !strings.HasSuffix(path, string(os.PathSeparator)) {
		path += string(os.PathSeparator)
	}
	return path
}
