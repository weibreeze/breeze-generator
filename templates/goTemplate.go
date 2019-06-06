package templates

import (
	"breeze-generator/core"
	"bytes"
	"strconv"
	"strings"
)

const GoPackagePrefix = "go_package_prefix"

var (
	goTypes = map[int]*goTypeInfo{
		core.Bool:    {typeString: "bool"},
		core.String:  {typeString: "string"},
		core.Byte:    {typeString: "byte"},
		core.Bytes:   {typeString: "[]byte"},
		core.Int16:   {typeString: "int16"},
		core.Int32:   {typeString: "int32"},
		core.Int64:   {typeString: "int64"},
		core.Float32: {typeString: "float32"},
		core.Float64: {typeString: "float64"},
		core.Array:   {typeString: "[]"},
		core.Map:     {typeString: "map["},
	}
)

type goTypeInfo struct {
	typeString string
}

//GoTemplate : can generate golang code according to schema
type GoTemplate struct {
}

//Name : template name
func (gt *GoTemplate) Name() string {
	return Go
}

//GenerateCode : generate golang code, one schema one file
func (gt *GoTemplate) GenerateCode(schema *core.Schema, context *core.Context) (contents map[string][]byte, err error) {
	buf := &bytes.Buffer{}
	importStr := make([]string, 0, 8)
	importStr = append(importStr, "github.com/weibreeze/breeze-go")
	messages := sortMessages(schema)
	for _, message := range messages {
		if message.IsEnum {
			importStr, err = gt.generateEnum(schema, message, context, buf, importStr)
		} else {
			importStr, err = gt.generateMessage(schema, message, context, buf, importStr)
		}
		if err != nil {
			return nil, err
		}
	}
	if len(schema.Services) > 0 {
		for _, service := range schema.Services {
			importStr, err = gt.generateService(schema, service, context, buf, importStr)
			if err != nil {
				return nil, err
			}
		}
	}
	content := &bytes.Buffer{}
	writeGenerateComment(content, schema.Name)
	pkgIndex := strings.LastIndex(schema.Package, ".")
	pkg := "\npackage " + schema.Package[pkgIndex+1:] + "\n\n"
	content.WriteString(pkg)
	gt.writeGoImport(importStr, content)
	content.Write(buf.Bytes())

	//init method
	if len(messages) > 0 {
		content.WriteString("\n")
		for _, message := range messages {
			content.WriteString("var " + gt.schemaName(message.Name) + "  *breeze.Schema\n")
		}
		content.WriteString("\nfunc init() {\n")
		for _, message := range messages {
			content.WriteString("	" + gt.schemaName(message.Name) + " = &breeze.Schema{Name: \"" + schema.OrgPackage + "." + message.Name)
			if message.Alias != "" {
				content.WriteString("\", Alias: \"" + message.Alias)
			}
			content.WriteString("\"}\n")
			if message.IsEnum {
				content.WriteString("	" + gt.schemaName(message.Name) + ".PutFields(&breeze.Field{Index: 1, Name: \"enumNumber\", Type: \"int32\"})\n")
			} else { // message
				for _, field := range sortFields(message) {
					content.WriteString("	" + gt.schemaName(message.Name) + ".PutFields(&breeze.Field{Index: " + strconv.Itoa(field.Index) + ", Name: \"" + field.Name + "\", Type: \"" + field.Type.TypeString + "\"})\n")
				}
			}
			content.WriteString("\n")
		}
		content.Truncate(content.Len() - 1)
		content.WriteString("}\n")
	}

	contents = make(map[string][]byte)
	fileName := schema.Name
	var index int
	if index = strings.LastIndex(fileName, "."); index > -1 { //remove suffix of schema file.
		fileName = fileName[:index]
		if index = strings.LastIndex(fileName, "."); index > -1 { // remove package
			fileName = fileName[index+1:]
		}
	}
	fileName = withPackageDir(fileName, schema)
	contents[fileName+".go"] = content.Bytes()
	return contents, nil
}

func (gt *GoTemplate) generateMessage(schema *core.Schema, message *core.Message, context *core.Context, buf *bytes.Buffer, importStr []string) ([]string, error) {
	buf.WriteString("type " + message.Name + " struct {\n")
	fields := sortFields(message) //sorted fields
	for _, field := range fields {
		buf.WriteString("	" + firstUpper(field.Name) + " " + gt.getTypeString(field.Type) + "\n")
	}
	buf.WriteString("}\n\n")

	//writeTo
	shortName := strings.ToLower(message.Name[:1])
	funcName := "func (" + shortName + " *" + message.Name + ")"
	buf.WriteString(funcName + " WriteTo(buf *breeze.Buffer) error {\n	return breeze.WriteMessage(buf, " + shortName + ".GetName(), func(funcBuf *breeze.Buffer) {\n")
	for _, field := range fields {
		switch field.Type.Number {
		case core.String:
			buf.WriteString("		if " + shortName + "." + firstUpper(field.Name) + " != \"\" {\n")
		case core.Byte, core.Int16, core.Int32, core.Int64, core.Float32, core.Float64:
			buf.WriteString("		if " + shortName + "." + firstUpper(field.Name) + " != 0 {\n")
		case core.Map, core.Array, core.Bytes:
			buf.WriteString("		if len(" + shortName + "." + firstUpper(field.Name) + ") > 0 {\n")
		case core.Bool:
			buf.WriteString("		if " + shortName + "." + firstUpper(field.Name) + " {\n")
		default: // message
			buf.WriteString("		if " + shortName + "." + firstUpper(field.Name) + " != nil {\n")
		}
		buf.WriteString("			breeze.WriteMessageField(funcBuf, " + strconv.Itoa(field.Index) + ", " + shortName + "." + firstUpper(field.Name) + ")\n		}\n")
	}
	buf.WriteString("	})\n}\n\n")

	//readFrom
	buf.WriteString(funcName + " ReadFrom(buf *breeze.Buffer) error {\n		return breeze.ReadMessageByField(buf, func(funcBuf *breeze.Buffer, index int) (err error) {\n		switch index {\n")
	for _, field := range fields {
		buf.WriteString("		case " + strconv.Itoa(field.Index) + ":\n")
		tp := field.Type
		switch tp.Number {
		case core.Map:
			buf.WriteString("			" + shortName + "." + firstUpper(field.Name) + " = make(" + gt.getTypeString(tp) + ", 16)\n")
		case core.Array:
			buf.WriteString("			" + shortName + "." + firstUpper(field.Name) + " = make(" + gt.getTypeString(tp) + ", 0, 16)\n")
		case core.Msg:
			if isEnum(field.Type, schema, context) {
				buf.WriteString("			var value " + gt.getTypeString(tp)[1:] + "\n			_, err = breeze.ReadValue(funcBuf, &value)\n")
				buf.WriteString("			" + shortName + "." + firstUpper(field.Name) + " = &value\n")
			} else {
				buf.WriteString("			" + shortName + "." + firstUpper(field.Name) + " = &" + gt.getTypeString(tp)[1:] + "{}\n")
				buf.WriteString("			_, err = breeze.ReadValue(funcBuf, " + shortName + "." + firstUpper(field.Name) + ")\n")
			}
			continue
		default:
		}
		buf.WriteString("			_, err = breeze.ReadValue(funcBuf, &" + shortName + "." + firstUpper(field.Name) + ")\n")
	}
	buf.WriteString("		default: //skip unknown field\n			_, err = breeze.ReadValue(funcBuf, nil)\n		}\n		return err\n	})\n}\n\n")

	//interface methods
	gt.addCommonInterfaceMethod(funcName, gt.schemaName(message.Name), buf)
	return importStr, nil
}

func (gt *GoTemplate) generateEnum(schema *core.Schema, message *core.Message, context *core.Context, buf *bytes.Buffer, importStr []string) ([]string, error) {
	importStr = append(importStr, "errors", "strconv")
	// const
	buf.WriteString("\nconst (\n")
	fields := sortEnumValues(message) //sorted enum values
	for _, v := range fields {
		buf.WriteString("	" + message.Name + firstUpper(v.Name) + " " + message.Name + " = " + strconv.Itoa(v.Index) + "\n")
	}
	buf.WriteString(")\n\n")

	// type define
	buf.WriteString("type " + message.Name + " int\n")

	// write to
	shortName := strings.ToLower(message.Name[:1])
	funcName := "func (" + shortName + " " + message.Name + ")" // not address method
	buf.WriteString(funcName + " WriteTo(buf *breeze.Buffer) error {\n	return breeze.WriteMessage(buf, " + shortName + ".GetName(), func(funcBuf *breeze.Buffer) {\n")
	buf.WriteString("		breeze.WriteMessageField(funcBuf, 1, int(" + shortName + "))\n	})\n}\n\n")

	// read from
	buf.WriteString(funcName + " ReadFrom(buf *breeze.Buffer) error {\n	return errors.New(\"can not read enum by Message.ReadFrom, Enum.ReadEnum is expected. name:\" + " + shortName + ".GetName())\n}\n\n")

	// read enum
	buf.WriteString(funcName + " ReadEnum(buf *breeze.Buffer, asAddr bool) (breeze.Enum, error) {\n	var number int\n	e := breeze.ReadMessageByField(buf, func(funcBuf *breeze.Buffer, index int) (err error) {\n")
	buf.WriteString("		switch index {\n		case 1:\n			err = breeze.ReadInt(buf, &number)\n")
	buf.WriteString("		default: //skip unknown field\n			_, err = breeze.ReadValue(funcBuf, nil)\n		}\n		return err\n	})\n")
	buf.WriteString("	if e == nil {\n		var result " + message.Name + "\n		switch number {\n")
	for _, v := range fields {
		buf.WriteString("		case " + strconv.Itoa(v.Index) + ":\n			result = " + message.Name + firstUpper(v.Name) + "\n")
	}
	buf.WriteString("		default:\n			return nil, errors.New(\"unknown enum number \" + strconv.Itoa(number))\n		}\n		if asAddr {\n			return &result, nil\n		}\n		return result, nil\n	}\n	return nil, e\n}\n\n")

	gt.addCommonInterfaceMethod(funcName, gt.schemaName(message.Name), buf)
	return importStr, nil
}

func (gt *GoTemplate) getTypeImport(tp *core.Type, tps []string, context *core.Context) []string {
	switch tp.Number {
	case core.Array, core.Map: //only array or map value maybe contains message type
		tps = gt.getTypeImport(tp.ValueType, tps, context)
	case core.Msg:
		index := strings.LastIndex(tp.Name, ".")
		if index > -1 { //not same package
			prefix := ""
			if context.Options != nil {
				prefix = context.Options[GoPackagePrefix]
			}
			tps = append(tps, prefix+strings.ReplaceAll(tp.Name[:index], ".", "/"))
		}
	}
	return tps
}

func (gt *GoTemplate) getTypeString(tp *core.Type) string {
	if tp.Number < core.Map {
		return goTypes[tp.Number].typeString
	}
	switch tp.Number {
	case core.Array:
		return goTypes[tp.Number].typeString + gt.getTypeString(tp.ValueType)
	case core.Map:
		return goTypes[tp.Number].typeString + gt.getTypeString(tp.KeyType) + "]" + gt.getTypeString(tp.ValueType)
	case core.Msg:
		index := strings.LastIndex(tp.Name, ".")
		if index > -1 { //not same package
			return "*" + tp.Name[strings.LastIndex(tp.Name[:index], ".")+1:]
		}
		return "*" + tp.Name
	}
	return ""
}

func (gt *GoTemplate) generateService(schema *core.Schema, service *core.Service, context *core.Context, buf *bytes.Buffer, importStr []string) ([]string, error) {
	//TODO implement
	return importStr, nil
}

func (gt *GoTemplate) generateMotanClient(schema *core.Schema, service *core.Service, context *core.Context, buf *bytes.Buffer) (importStr []string, err error) {
	//TODO implement
	return nil, nil
}

func (gt *GoTemplate) schemaName(name string) string {
	return firstLower(name) + "BreezeSchema"
}

func (gt *GoTemplate) addCommonInterfaceMethod(funcName string, schemaName string, buf *bytes.Buffer) {
	buf.WriteString(funcName + " GetName() string {\n	return " + schemaName + ".Name\n}\n\n")
	buf.WriteString(funcName + " GetAlias() string {\n	return " + schemaName + ".Alias\n}\n\n")
	buf.WriteString(funcName + " GetSchema() *breeze.Schema {\n	return " + schemaName + "\n}\n\n")
}

func (gt *GoTemplate) writeGoImport(importStrs []string, buf *bytes.Buffer) {
	sys := make([]string, 0, 16)
	out := make([]string, 0, 16)
	for _, value := range importStrs {
		if strings.Contains(value, "/") {
			out = append(out, value)
		} else {
			sys = append(sys, value)
		}
	}
	buf.WriteString("import (\n")
	for _, value := range sortUnique(sys) {
		buf.WriteString("	\"" + value + "\"\n")
	}
	buf.WriteString("\n")
	for _, value := range sortUnique(out) {
		buf.WriteString("	\"" + value + "\"\n")
	}
	buf.WriteString(")\n\n")
}
