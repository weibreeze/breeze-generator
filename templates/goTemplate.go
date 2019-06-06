package templates

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/weibreeze/breeze-generator/core"
)

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
	if len(schema.Messages) > 0 {
		for _, message := range schema.Messages {
			importStr, err = gt.generateMessage(schema, message, context, buf, importStr)
			if err != nil {
				return nil, err
			}
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
	content.WriteString("import (\n")
	importStr = sortUnique(importStr)
	for _, s := range importStr {
		content.WriteString("	\"" + s + "\"\n")
	}
	content.WriteString(")\n\n")
	content.Write(buf.Bytes())

	//init method
	if len(schema.Messages) > 0 {
		content.WriteString("\n")
		for name := range schema.Messages {
			content.WriteString("var " + gt.schemaName(name) + "  *breeze.Schema\n")
		}
		content.WriteString("\nfunc init() {\n")
		for name, message := range schema.Messages {
			content.WriteString("	" + gt.schemaName(name) + " = &breeze.Schema{Name: \"" + schema.Package + "." + name)
			if message.Alias != "" {
				content.WriteString("\", Alias: \"" + message.Alias)
			}
			content.WriteString("\"}\n")
			for _, field := range sortFields(message) {
				content.WriteString("	" + gt.schemaName(name) + ".PutFields(&breeze.Field{Index: " + strconv.Itoa(field.Index) + ", Name: \"" + field.Name + "\", Type: \"" + field.Type.TypeString + "\"})\n")
			}
			content.WriteString("\n")
		}
		content.WriteString("}\n")
	}

	contents = make(map[string][]byte)
	fileName := schema.Name
	if strings.LastIndex(schema.Name, ".") > -1 { //remove suffix of schema file.
		fileName = fileName[:strings.LastIndex(schema.Name, ".")]
	}
	fileName = withPackageDir(fileName, schema)
	contents[fileName+".go"] = content.Bytes()
	return contents, nil
}

func (gt *GoTemplate) generateMessage(schema *core.Schema, message *core.Message, context *core.Context, buf *bytes.Buffer, importStr []string) ([]string, error) {
	buf.WriteString("type " + message.Name + " struct {\n")
	fields := sortFields(message) //sorted fields
	for _, field := range fields {
		importStr = gt.getTypeImport(field.Type, importStr)
		buf.WriteString("	" + firstUpper(field.Name) + " " + gt.getTypeString(field.Type) + "\n")
	}
	buf.WriteString("}\n\n")

	//writeTo
	shortName := strings.ToLower(message.Name[:1])
	funcName := "func (" + shortName + " *" + message.Name + ")"
	buf.WriteString(funcName + " WriteTo(buf *breeze.Buffer) error {\n	return breeze.WriteMessage(buf, " + shortName + ".GetName(), func(funcBuf *breeze.Buffer) {\n")
	for _, field := range fields {
		buf.WriteString("		breeze.WriteMessageField(funcBuf, " + strconv.Itoa(field.Index) + ", " + shortName + "." + firstUpper(field.Name) + ")\n")
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
		default:
		}
		buf.WriteString("			_, err = breeze.ReadValue(funcBuf, &" + shortName + "." + firstUpper(field.Name) + ")\n")
	}
	buf.WriteString("		default: //skip unknown field\n			_, err = breeze.ReadValue(funcBuf, nil)\n		}\n		return err\n	})\n}\n\n")

	//interface methods
	schemaName := gt.schemaName(message.Name)
	buf.WriteString(funcName + " GetName() string {\n	return " + schemaName + ".Name\n}\n\n")
	buf.WriteString(funcName + " GetAlias() string {\n	return " + schemaName + ".Alias\n}\n\n")
	buf.WriteString(funcName + " GetSchema() *breeze.Schema {\n	return " + schemaName + "\n}\n\n")
	return importStr, nil
}

func (gt *GoTemplate) getTypeImport(tp *core.Type, tps []string) []string {
	switch tp.Number {
	case core.Array, core.Map: //only array or map value maybe contains message type
		tps = gt.getTypeImport(tp.ValueType, tps)
	case core.Msg:
		index := strings.LastIndex(tp.Name, ".")
		if index > -1 { //not same package
			tps = append(tps, strings.ReplaceAll(tp.Name[:index], ".", "/"))
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
