package templates

import (
	"breeze-generator/core"
	"bytes"
	"strconv"
	"strings"
)

var (
	javaTypes = map[int]*javaTypeInfo{
		core.Bool:    {typeString: "boolean", wrapperTypeString: "Boolean", className: "boolean.class"},
		core.String:  {typeString: "String", wrapperTypeString: "String", className: "String.class"},
		core.Byte:    {typeString: "byte", wrapperTypeString: "Byte", className: "byte.class"},
		core.Bytes:   {typeString: "byte[]", wrapperTypeString: "byte[]", className: "byte[].class"},
		core.Int16:   {typeString: "short", wrapperTypeString: "Short", className: "short.class"},
		core.Int32:   {typeString: "int", wrapperTypeString: "Integer", className: "int.class"},
		core.Int64:   {typeString: "long", wrapperTypeString: "Long", className: "long.class"},
		core.Float32: {typeString: "float", wrapperTypeString: "Float", className: "float.class"},
		core.Float64: {typeString: "double", wrapperTypeString: "Double", className: "double.class"},
		core.Array:   {typeString: "List<", className: "List.class"},
		core.Map:     {typeString: "Map<", className: "Map.class"},
	}
)

type javaTypeInfo struct {
	typeString        string
	wrapperTypeString string
	className         string
}

//JavaTemplate : can generate java code according to schema
type JavaTemplate struct {
}

//Name : template name
func (jt *JavaTemplate) Name() string {
	return Java
}

//GenerateCode : generate java code
func (jt *JavaTemplate) GenerateCode(schema *core.Schema, context *core.Context) (contents map[string][]byte, err error) {
	contents = make(map[string][]byte)
	if len(schema.Messages) > 0 {
		for _, message := range schema.Messages {
			file, content, err := jt.generateMessage(schema, message, context)
			if err != nil {
				return nil, err
			}
			if file != "" && content != nil {
				contents[file] = content
			}
		}
	}
	if len(schema.Services) > 0 {
		for _, service := range schema.Services {
			file, content, err := jt.generateService(schema, service, context)
			if err != nil {
				return nil, err
			}
			if file != "" && content != nil {
				contents[file] = content
			}
		}
	}
	return contents, nil
}

func (jt *JavaTemplate) generateMessage(schema *core.Schema, message *core.Message, context *core.Context) (file string, content []byte, err error) {
	buf := &bytes.Buffer{}
	writeGenerateComment(buf, schema.Name)
	pkg := schema.Options[core.JavaPackage]
	if pkg == "" {
		pkg = schema.Package
	}
	buf.WriteString("package " + pkg + ";\n\n")
	//import
	buf.WriteString("import com.weibo.breeze.*;\nimport com.weibo.breeze.message.Message;\nimport com.weibo.breeze.message.Schema;\n\n")

	fields := sortFields(message) //sorted fields

	importStr := make([]string, 0, 16)
	for _, field := range fields {
		importStr = jt.getTypeImport(field.Type, context, importStr)
	}
	if len(importStr) > 0 {
		importStr = sortUnique(importStr)
		for _, t := range importStr {
			buf.WriteString(t)
		}
		buf.WriteString("\n")
	}
	buf.WriteString("import java.lang.reflect.Type;\n")
	buf.WriteString("import java.util.*;\n\n")

	//class body
	buf.WriteString("public class " + message.Name + " implements Message {\n    private static final Schema schema = new Schema();\n    private static final Map<String, Type> genericTypes = new HashMap<>();\n")
	for _, field := range fields {
		buf.WriteString("    private " + jt.getTypeString(field.Type, false) + " " + field.Name + ";\n")
	}
	buf.WriteString("\n    static {\n        try {\n            schema.setName(\"" + schema.Package + "." + message.Name + "\")")
	for _, field := range fields {
		buf.WriteString("\n                    .putField(new Schema.Field(" + strconv.Itoa(field.Index) + ", \"" + field.Name + "\", \"" + field.Type.TypeString + "\"))")
	}
	buf.WriteString(";\n        } catch (BreezeException ignore) {}\n        Breeze.putMessageInstance(schema.getName(), new " + message.Name + "());\n")

	for _, field := range fields {
		if field.Type.Number == core.Map || field.Type.Number == core.Array {
			buf.WriteString("        Breeze.addGenericType(genericTypes, " + message.Name + ".class, \"" + field.Name + "\");\n")
		}
	}
	buf.WriteString("    }\n\n")

	//writeTo
	buf.WriteString("    @Override\n    public void writeToBuf(BreezeBuffer buffer) throws BreezeException {\n        BreezeWriter.writeMessage(buffer, schema.getName(), () -> {\n")
	for _, field := range fields {
		buf.WriteString("            BreezeWriter.writeMessageField(buffer, " + strconv.Itoa(field.Index) + ", " + field.Name + ");\n")
	}
	buf.WriteString("        });\n    }\n\n")

	//readFrom
	buf.WriteString("    @Override\n    public Message readFromBuf(BreezeBuffer buffer) throws BreezeException {\n        BreezeReader.readMessage(buffer, false, (int index) -> {\n            switch (index) {\n")
	for _, field := range fields {
		tp := field.Type
		buf.WriteString("                case " + strconv.Itoa(field.Index) + ":\n                    ")
		switch tp.Number {
		case core.Map:
			buf.WriteString(field.Name + " = new HashMap<>();\n                    BreezeReader.readMapByType(buffer, " + field.Name + ", genericTypes.get(\"" + field.Name + "\" + Breeze.KEY_TYPE_SUFFIX), genericTypes.get(\"" + field.Name + "\" + Breeze.VALUE_TYPE_SUFFIX));\n")
		case core.Array:
			buf.WriteString(field.Name + " = new ArrayList<>();\n                    BreezeReader.readCollectionByType(buffer, " + field.Name + ", genericTypes.get(\"" + field.Name + "\" + Breeze.VALUE_TYPE_SUFFIX));\n")
		case core.Msg:
			buf.WriteString(field.Name + " = BreezeReader.readObject(buffer, " + tp.Name + ".class);\n")
		default:
			buf.WriteString(field.Name + " = BreezeReader.readObject(buffer, " + javaTypes[tp.Number].className + ");\n")
		}
		buf.WriteString("                    break;\n")
	}
	buf.WriteString("                default: //skip unknown field\n                    BreezeReader.readObject(buffer, Object.class);\n            }\n        });\n        return this;\n    }\n\n")

	//interface methods
	buf.WriteString("    @Override\n    public String getName() { return schema.getName(); }\n\n")
	buf.WriteString("    @Override\n    public String getAlias() { return schema.getAlias(); }\n\n")
	buf.WriteString("    @Override\n    public Schema getSchema() { return schema; }\n\n")
	buf.WriteString("    @Override\n    public Message getDefaultInstance() { return new " + message.Name + "(); }\n\n")

	//setter and getter
	for _, field := range fields {
		buf.WriteString("    public " + jt.getTypeString(field.Type, false) + " get" + firstUpper(field.Name) + "() { return " + field.Name + "; }\n\n")
		buf.WriteString("    public void set" + firstUpper(field.Name) + "(" + jt.getTypeString(field.Type, false) + " " + field.Name + ") { this." + field.Name + " = " + field.Name + "; }\n\n")
	}
	buf.WriteString("}\n")

	return withPackageDir(message.Name, schema) + ".java", buf.Bytes(), nil
}

func (jt *JavaTemplate) getTypeImport(tp *core.Type, context *core.Context, tps []string) []string {
	switch tp.Number {
	case core.Array, core.Map: //only array or map value maybe contains message type
		tps = jt.getTypeImport(tp.ValueType, context, tps)
	case core.Msg:
		index := strings.LastIndex(tp.Name, ".")
		if index > -1 { //not same package
			if msg, ok := context.Messages[tp.Name]; ok {
				pkg := msg.Options[core.JavaPackage]
				if pkg != "" {
					tps = append(tps, "import "+pkg+"."+tp.Name[index+1:]+";\n")
					return tps
				}
			}
			tps = append(tps, "import "+tp.Name+";\n")
		}
	}
	return tps
}

func (jt *JavaTemplate) getTypeString(tp *core.Type, wrapper bool) string {
	if tp.Number < core.Map {
		if wrapper {
			return javaTypes[tp.Number].wrapperTypeString
		}
		return javaTypes[tp.Number].typeString
	}
	switch tp.Number {
	case core.Array:
		return javaTypes[tp.Number].typeString + jt.getTypeString(tp.ValueType, true) + ">"
	case core.Map:
		return javaTypes[tp.Number].typeString + jt.getTypeString(tp.KeyType, true) + ", " + jt.getTypeString(tp.ValueType, true) + ">"
	case core.Msg:
		return tp.Name[strings.LastIndex(tp.Name, ".")+1:]
	}
	return ""
}

func (jt *JavaTemplate) generateService(schema *core.Schema, service *core.Service, context *core.Context) (file string, content []byte, err error) {
	//TODO implement
	return "", nil, nil
}

func (jt *JavaTemplate) generateMotanClient(schema *core.Schema, service *core.Service, context *core.Context) (file string, content []byte, err error) {
	//TODO implement
	return "", nil, nil
}
