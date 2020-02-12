package pkg

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"github.com/weibreeze/breeze-generator/pkg/core"
	"github.com/weibreeze/breeze-generator/pkg/parsers"
	"github.com/weibreeze/breeze-generator/pkg/templates"
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
	fstat, err := os.Stat(path)
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
	if fstat.IsDir() {
		optionsFileName := path + core.PathSeparator + context.Parser.Name() + ".options"
		err := initContextOptions(optionsFileName, context)
		if err != nil {
			return nil, err
		}
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

func parseOptions(reader io.Reader) (map[string]string, error) {
	options := make(map[string]string, 16)
	optionsReader := bufio.NewReader(reader)
	convert := func(input string) (string, error) {
		// convert unicode chars and saved chars
		outputBuf := bytes.Buffer{}
		inputRunes := []rune(input)
		inputLen := len(inputRunes)
		for i := 0; i < inputLen; i++ {
			ch := inputRunes[i]
			if ch != '\\' {
				outputBuf.WriteRune(ch)
			}
			if i+1 >= inputLen {
				outputBuf.WriteRune(ch)
				continue
			}
			i++
			ch = inputRunes[i]
			if ch == 'u' {
				if i+4 >= inputLen {
					return "", fmt.Errorf("malformed \\uxxxx encoding of input %s", input)
				}
				var value rune = 0
				for j := 1; j <= 4; j++ {
					ch = inputRunes[i+j]
					switch ch {
					case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
						value = (value << 4) + ch - '0'
					case 'a', 'b', 'c', 'd', 'e', 'f':
						value = (value << 4) + 10 + ch - 'a'
					case 'A', 'B', 'C', 'D', 'E', 'F':
						value = (value << 4) + 10 + ch - 'A'
					default:
						return "", fmt.Errorf("malformed \\uxxxx encoding of input %s", input)
					}
				}
				i += 4
				outputBuf.WriteRune(value)
			} else if ch == 't' {
				outputBuf.WriteRune('\t')
			} else if ch == 'r' {
				outputBuf.WriteRune('\r')
			} else if ch == 'n' {
				outputBuf.WriteRune('\n')
			} else if ch == 'f' {
				outputBuf.WriteRune('\f')
			} else {
				outputBuf.WriteRune(ch)
			}
		}
		return outputBuf.String(), nil
	}

	for {
		line, err := optionsReader.ReadString('\n')
		if err != nil && err != io.EOF {
			return options, err
		}
		if err == io.EOF && line == "" {
			return options, nil
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx == -1 {
			// no value ignore
			continue
		}
		key, err := convert(strings.TrimSpace(line[:idx]))
		if err != nil {
			return options, err
		}
		value, err := convert(strings.TrimSpace(line[idx+1:]))
		if err != nil {
			return options, err
		}
		options[key] = value
	}
}

func parseSchemaWithPath(path string, context *core.Context) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		var fileInfo []os.FileInfo
		fileInfo, err = ioutil.ReadDir(path)
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
	for _, template := range context.Templates {
		for _, schema := range context.Schemas {
			files, err := template.GenerateCode(schema, context)
			if err != nil {
				fmt.Printf("error: generate code fail, template:%s, err:%s\n", template.Name(), err.Error())
				continue
			}
			path := context.WritePath
			if path[len(path)-1:] != core.PathSeparator {
				path += core.PathSeparator
			}
			path = path + template.Name() + core.PathSeparator
			err = os.MkdirAll(path, core.DefaultNewDirectoryMode)
			if err != nil {
				return err
			}
			for name, content := range files {
				index := strings.LastIndex(name, core.PathSeparator) //contains path
				if index > -1 {
					err := os.MkdirAll(path+name[:index+1], core.DefaultNewDirectoryMode)
					if err != nil {
						return err
					}
				}
				err = ioutil.WriteFile(path+name, content, core.DefaultNewRegularFileMode)
				if err != nil {
					fmt.Printf("error: write code fail, template:%s, file name:%s, err:%s\n", template.Name(), name, err.Error())
				}
			}
		}
		err := template.PostAllGenerated(context)
		if err != nil {
			fmt.Printf("error: post generated handle fail, template: %s, err: %s\n", template.Name(), err.Error())
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

func initContextOptions(optionsFileName string, context *core.Context) error {
	if _, err := os.Stat(optionsFileName); err == nil {
		f, err := os.Open(optionsFileName)
		if err != nil {
			return err
		}
		defer f.Close()
		options, err := parseOptions(f)
		if err != nil {
			return err
		}
		for k, v := range options {
			if _, exist := context.Options[k]; exist {
				continue
			}
			context.Options[k] = v
		}
	}
	return nil
}

func addSeparator(path string) string {
	if !strings.HasSuffix(path, string(os.PathSeparator)) {
		path += string(os.PathSeparator)
	}
	return path
}
