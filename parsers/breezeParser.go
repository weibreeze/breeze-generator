package parsers

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/weibreeze/breeze-generator/core"
)

//keywords
const (
	Option  = "option"
	Message = "message"
	Package = "package"
	Service = "service"
	Enum    = "enum"
)

var (
	regPackage = regexp.MustCompile("^[\\w.]+$")
	regName    = regexp.MustCompile("^[\\w]+$")
	regField   = regexp.MustCompile("^([\\w<>., ]+) +(\\w+) *= *(\\d+) *$")
)

//BreezeParser can parse a schema according to breeze specification
type BreezeParser struct {
}

//Name : parse _name
func (b *BreezeParser) Name() string {
	return Breeze
}

//FileSuffix : suffix of file which can be processed by this Parser
func (b *BreezeParser) FileSuffix() string {
	return BreezeFileSuffix
}

//ParseSchema : parse and return a breeze schema
func (b *BreezeParser) ParseSchema(content []byte, context *core.Context) (schema *core.Schema, err error) {
	buf := bytes.NewBuffer(content)
	var line string
	schema = &core.Schema{Options: make(map[string]string), Messages: make(map[string]*core.Message), Services: make(map[string]*core.Service)}
	for {
		line, err = readCleanLine(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if line == "" && err == io.EOF {
			return schema, nil
		}
		err = process(line, buf, schema)
		if err != nil {
			if err == io.EOF {
				return schema, nil
			}
			return nil, err
		}
	}
}

func process(line string, buf *bytes.Buffer, schema *core.Schema) error {
	if line == "" {
		return nil
	}

	switch getTag(line) {
	case Option:
		k, v, err := parseOption(line)
		if err != nil {
			return err
		}
		schema.Options[k] = v
	case Package:
		schema.OrgPackage = strings.TrimSpace(line[len(Package):])
		schema.Package = schema.OrgPackage
		if !regPackage.MatchString(schema.OrgPackage) {
			return errors.New("package _name illegal. package: " + schema.OrgPackage)
		}
		if UniformPackage != "" { // file package
			schema.Package = UniformPackage
		}
	case Message:
		msg, err := parseMessage(buf, line)
		if err != nil {
			return err
		}
		schema.Messages[msg.Name] = msg
	case Service:
		service, err := parseService(buf, line)
		if err != nil {
			return err
		}
		schema.Services[service.Name] = service
	case Enum:
		msg, err := parseEnum(buf, line)
		if err != nil {
			return err
		}
		schema.Messages[msg.Name] = msg
	}
	return nil
}

func readCleanLine(buf *bytes.Buffer) (line string, err error) {
	line, err = buf.ReadString('\n')
	if len(line) > 0 {
		//remove end tag
		index := strings.Index(line, ";")
		if index > -1 {
			line = line[0:index]
		}

		//remove comment in line which not have end tag
		index = strings.Index(line, "//")
		if index > -1 {
			line = line[0:index]
		}
		//trim
		line = strings.TrimSpace(line)
	}
	return line, err
}

func parseOption(line string) (key string, value string, err error) {
	str := strings.Split(line[len(Option):], "=")
	if len(str) != 2 {
		return key, value, errors.New("wrong option line: " + line)
	}
	return strings.TrimSpace(str[0]), strings.TrimSpace(str[1]), nil
}

func parseMessage(buf *bytes.Buffer, firstLine string) (message *core.Message, err error) {
	message = &core.Message{Fields: make(map[int]*core.Field), Options: make(map[string]string)}
	name, options, err := parseSegment(buf, firstLine, len(Message), func(line string) error {
		field, innerErr := parseField(line)
		if innerErr != nil {
			return innerErr
		}
		if field != nil {
			message.Fields[field.Index] = field
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(message.Fields) == 0 {
		return nil, errors.New("message field is empty. message: " + name)
	}
	message.Name = name
	message.Options = options
	if options[core.Alias] != "" {
		message.Alias = options[core.Alias]
	}
	return message, nil
}

func parseEnum(buf *bytes.Buffer, firstLine string) (message *core.Message, err error) {
	message = &core.Message{EnumValues: make(map[int]string), Options: make(map[string]string), IsEnum: true}
	name, options, err := parseSegment(buf, firstLine, len(Enum), func(line string) error {
		strs := strings.Split(line, "=")
		if len(strs) != 2 {
			return errors.New("wrong enum format. line:" + line)
		}
		index, err := strconv.Atoi(strings.TrimSpace(strs[1]))
		if err != nil {
			return errors.New("wrong enum index. line:" + line)
		}
		message.EnumValues[index] = strings.TrimSpace(strs[0])
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(message.EnumValues) == 0 {
		return nil, errors.New("enum _value is empty. enum: " + name)
	}
	message.Name = name
	message.Options = options
	if options[core.Alias] != "" {
		message.Alias = options[core.Alias]
	}
	return message, nil
}

func parseField(line string) (field *core.Field, err error) {
	result := regField.FindStringSubmatchIndex(line)
	if len(result) < 8 {
		return nil, errors.New("wrong field format. line:" + line)
	}
	tag := line[result[2]:result[3]]
	tp, err := core.GetType(tag, UniformPackage != "")
	if err != nil {
		return nil, err
	}
	name := line[result[4]:result[5]]
	index, err := strconv.Atoi(line[result[6]:result[7]])
	if err != nil {
		return nil, err
	}
	return &core.Field{Name: name, Type: tp, Index: index}, nil
}

func parseSegment(buf *bytes.Buffer, firstLine string, start int, parseLine func(string) error) (name string, options map[string]string, err error) {
	options = make(map[string]string)
	//parse segment header
	index := strings.Index(firstLine, "(")
	if index > -1 { //has options
		name = firstLine[start:index]
		end := strings.Index(firstLine, ")")
		if end < 0 {
			return "", nil, errors.New("wrong format. line:" + firstLine)
		}
		items := strings.Split(firstLine[index+1:end], ",")
		for _, item := range items {
			op := strings.Split(item, "=")
			if len(op) != 2 {
				return "", nil, errors.New("wrong options format. line:" + firstLine)
			}
			options[strings.TrimSpace(op[0])] = strings.TrimSpace(op[1])
		}
	} else if index = strings.Index(firstLine, "{"); index > -1 { //end with '{'
		name = firstLine[start:index]
	} else {
		name = firstLine[start:]
	}
	name = strings.TrimSpace(name)
	if !regName.MatchString(name) {
		return "", nil, errors.New("segment _name illegal. line:" + firstLine)
	}
	name = strings.ToUpper(name[:1]) + name[1:]

	//parse segment line
	var line string
	for {
		line, err = readCleanLine(buf)
		if err != nil && err != io.EOF {
			return "", nil, err
		}
		if line == "" && err == io.EOF {
			return "", nil, errors.New("unexpected segment end. _name:" + name)
		}
		if line != "" {
			switch line {
			case "}": //expect segment end
				return name, options, nil
			case "{":
				continue
			default:
				err = parseLine(line)
				if err != nil {
					return "", nil, err
				}
			}
		}
	}
}

func parseService(buf *bytes.Buffer, firstLine string) (service *core.Service, err error) {
	service = &core.Service{Methods: make(map[string]*core.Method), Options: make(map[string]string)}
	name, options, err := parseSegment(buf, firstLine, len(Service), func(line string) error {
		method, innerErr := parseMethod(line)
		if innerErr != nil {
			return innerErr
		}
		if method != nil {
			service.Methods[method.Name] = method
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	service.Name = name
	service.Options = options
	return service, nil
}

func parseMethod(line string) (method *core.Method, err error) {
	index1 := strings.Index(line, "(")
	index2 := strings.Index(line, ")")
	if index1 < 0 || index2 < 0 || index2 < index1 {
		return nil, errors.New("wrong method format. line: " + line)
	}
	name := strings.TrimSpace(line[:index1])
	if !regName.MatchString(name) {
		return nil, errors.New("method _name illegal. line: " + line)
	}

	paramStr := strings.TrimSpace(line[index1+1 : index2])
	params := make(map[int]*core.Param, 16)
	if paramStr != "" {
		paramIndex := 0
		for len(paramStr) > 0 {
			index, err := getTypeStr(paramStr)
			if err != nil {
				return nil, err
			}
			tp := strings.TrimSpace(paramStr[:index])
			paramStr = paramStr[index:]
			index = strings.Index(paramStr, ",")
			if index < 0 {
				params[paramIndex] = &core.Param{Type: tp, Name: strings.TrimSpace(paramStr)}
				break
			}
			params[paramIndex] = &core.Param{Type: tp, Name: strings.TrimSpace(paramStr[:index])}
			paramStr = paramStr[index+1:]
			paramIndex++
		}
	}
	ret := strings.TrimSpace(line[index2+1:])
	method = &core.Method{Name: name, Return: ret, Params: params}
	return method, nil
}

func getTag(line string) string {
	return line[0:strings.Index(line, " ")]
}

func getTypeStr(str string) (index int, err error) {
	result := len(str) - len(strings.TrimLeftFunc(str, unicode.IsSpace))
	if strings.HasPrefix(str[result:], "map<") {
		result += 4
		index, err = getTypeStr(str[result:]) //map key
		if err != nil {
			return 0, err
		}
		result += index
		index = strings.Index(str[result:], ",")
		if index < 0 {
			return 0, errors.New("wrong map format:" + str)
		}
		result += index
		index, err = getTypeStr(str[result+1:]) //map _value
		if err != nil {
			return 0, err
		}
		result += index
		result += strings.Index(str[result:], ">") + 1
	} else if strings.HasPrefix(str[result:], "array<") {
		result += 6
		index, err = getTypeStr(str[result:]) //array _value
		if err != nil {
			return 0, err
		}
		result += index
		result += strings.Index(str[result:], ">") + 1
	} else {
		result += strings.Index(str[result:], " ")
	}
	return result, nil
}
