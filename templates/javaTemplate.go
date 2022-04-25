package templates

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/weibreeze/breeze-generator/core"
)

var (
	javaTypes = map[int]*javaTypeInfo{
		core.Bool:    {typeString: "boolean", wrapperTypeString: "Boolean", className: "boolean.class", breezeType: "TYPE_BOOL"},
		core.String:  {typeString: "String", wrapperTypeString: "String", className: "String.class", breezeType: "TYPE_STRING"},
		core.Byte:    {typeString: "byte", wrapperTypeString: "Byte", className: "byte.class", breezeType: "TYPE_BYTE"},
		core.Bytes:   {typeString: "byte[]", wrapperTypeString: "byte[]", className: "byte[].class", breezeType: "TYPE_BYTE_ARRAY"},
		core.Int16:   {typeString: "short", wrapperTypeString: "Short", className: "short.class", breezeType: "TYPE_INT16"},
		core.Int32:   {typeString: "int", wrapperTypeString: "Integer", className: "int.class", breezeType: "TYPE_INT32"},
		core.Int64:   {typeString: "long", wrapperTypeString: "Long", className: "long.class", breezeType: "TYPE_INT64"},
		core.Float32: {typeString: "float", wrapperTypeString: "Float", className: "float.class", breezeType: "TYPE_FLOAT32"},
		core.Float64: {typeString: "double", wrapperTypeString: "Double", className: "double.class", breezeType: "TYPE_FLOAT64"},
		core.Array:   {typeString: "List<", className: "List.class"},
		core.Map:     {typeString: "Map<", className: "Map.class"},
	}
)

type javaTypeInfo struct {
	typeString        string
	wrapperTypeString string
	className         string
	breezeType        string
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
			var file string
			var content []byte
			if message.IsEnum {
				file, content, err = jt.generateEnum(schema, message, context)
			} else {
				file, content, err = jt.generateMessage(schema, message, context)
			}
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
			file, content, err := jt.generateService(schema, service, context, false)
			if err != nil {
				return nil, err
			}
			if file != "" && content != nil {
				contents[file] = content
			}
			file, content, err = jt.generateService(schema, service, context, true)
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

func (jt *JavaTemplate) generateEnum(schema *core.Schema, message *core.Message, context *core.Context) (file string, content []byte, err error) {
	buf := &bytes.Buffer{}
	writeGenerateComment(buf, schema.Name)
	pkg := getJavaPkg(schema)
	buf.WriteString("package " + pkg + ";\n\n")
	//import
	buf.WriteString("import com.weibo.breeze.*;\nimport com.weibo.breeze.serializer.Serializer;\n\nimport static com.weibo.breeze.type.Types.TYPE_INT32;\n\n")

	enumValues := sortEnumValues(message) //sorted enumValues

	//class body
	buf.WriteString("public enum " + message.Name + " {\n")
	for _, value := range enumValues {
		buf.WriteString("    " + value.Name + "(" + strconv.Itoa(value.Index) + "),\n")
	}
	buf.Truncate(buf.Len() - 2)
	buf.WriteString(";\n\n")
	fullName := schema.OrgPackage + "." + message.Name
	buf.WriteString("    static {\n        try {\n            Breeze.registerSerializer(new " + message.Name + "Serializer());\n")
	buf.WriteString("        } catch (BreezeException ignore) {}\n    }\n\n")

	// enum number
	buf.WriteString("    private int number;\n\n")

	//constructor
	buf.WriteString("    " + message.Name + "(int number) { this.number = number; }\n\n")

	// enum serializer
	buf.WriteString("    public static class " + message.Name + "Serializer implements Serializer<" + message.Name + "> {\n")
	//names
	buf.WriteString("        private static final String[] names = new String[]{\"" + fullName + "\", " + message.Name + ".class.getName()};\n\n")

	//writeTo
	buf.WriteString("        @Override\n        public void writeToBuf(" + message.Name + " obj, BreezeBuffer breezeBuffer) throws BreezeException {\n")
	buf.WriteString("            BreezeWriter.writeMessage(breezeBuffer, () -> {\n                TYPE_INT32.writeMessageField(breezeBuffer, 1, obj.number);\n            });\n        }\n\n")

	//readFrom
	buf.WriteString("        @Override\n        public " + message.Name + " readFromBuf(BreezeBuffer breezeBuffer) throws BreezeException {\n            int[] number = new int[]{-1};\n")
	buf.WriteString("            BreezeReader.readMessage(breezeBuffer, (int breezeIndex) -> {\n                switch (breezeIndex) {\n")
	buf.WriteString("                    case 1:\n                        number[0] = TYPE_INT32.read(breezeBuffer);\n                        break;\n")
	buf.WriteString("                    default:\n                        BreezeReader.readObject(breezeBuffer, Object.class);\n                }\n            });\n")
	buf.WriteString("            switch (number[0]) {\n")
	for _, value := range enumValues {
		buf.WriteString("                case " + strconv.Itoa(value.Index) + ":\n                   return " + value.Name + ";\n")
	}
	buf.WriteString("            }\n            throw new BreezeException(\"unknown enum number:\" + number[0]);\n        }\n\n")

	//interface methods
	buf.WriteString("        @Override\n        public String[] getNames() { return names; }\n    }\n}\n")
	return withPackageDirByName(message.Name, schema, pkg, false) + ".java", buf.Bytes(), nil
}

func (jt *JavaTemplate) generateMessage(schema *core.Schema, message *core.Message, context *core.Context) (file string, content []byte, err error) {
	buf := &bytes.Buffer{}
	writeGenerateComment(buf, schema.Name)

	pkg := getJavaPkg(schema)
	// fix: none package in breeze, pkg is empty
	if pkg != "" {
		buf.WriteString("package " + pkg + ";\n\n")
	}

	fields := sortFields(message) //sorted fields

	//import
	importStr := make([]string, 0, 16)
	var needBreezeType bool
	for _, field := range fields { // message class import
		importStr = jt.getTypeImport(field.Type, context, importStr)
		if field.Type.Number >= core.Map { // map, array, message
			needBreezeType = true
		}
	}

	//breeze class import section
	buf.WriteString("import com.weibo.breeze.*;\nimport com.weibo.breeze.message.Message;\nimport com.weibo.breeze.message.Schema;\n")
	if needBreezeType {
		buf.WriteString("import com.weibo.breeze.type.BreezeType;\n")
	}
	buf.WriteString("\n")

	// message class import section
	if len(importStr) > 0 {
		importStr = sortUnique(importStr)
		for _, t := range importStr {
			buf.WriteString(t)
		}
		buf.WriteString("\n")
	}

	// static import
	if needBreezeType {
		buf.WriteString("import static com.weibo.breeze.Breeze.getBreezeType;\n")
	}
	buf.WriteString("import static com.weibo.breeze.type.Types.*;\n\n")

	//class body
	buf.WriteString("public class " + message.Name + " implements Message {\n    private static final Schema breezeSchema = new Schema();\n")
	//breezetype
	for _, field := range fields {
		if field.Type.Number >= core.Map {
			buf.WriteString("    private static BreezeType<" + jt.getTypeString(field.Type, false) + "> " + field.Name + "BreezeType;\n")
		}
	}

	// init schema
	for _, field := range fields {
		buf.WriteString("    private " + jt.getTypeString(field.Type, false) + " " + field.Name + ";\n")
	}
	buf.WriteString("\n    static {\n        try {\n            breezeSchema.setName(\"" + schema.OrgPackage + "." + message.Name + "\")")
	for _, field := range fields {
		buf.WriteString("\n                    .putField(new Schema.Field(" + strconv.Itoa(field.Index) + ", \"" + field.Name + "\", \"" + field.Type.TypeString + "\"))")
	}
	buf.WriteString(";\n")
	// init breeze type
	for _, field := range fields {
		if field.Type.Number >= core.Map {
			buf.WriteString("            " + field.Name + "BreezeType = getBreezeType(" + message.Name + ".class, \"" + field.Name + "\");\n")
		}
	}
	buf.WriteString("        } catch (BreezeException ignore) {}\n        Breeze.putMessageInstance(breezeSchema.getName(), new " + message.Name + "());\n    }\n\n")

	//writeTo
	buf.WriteString("    @Override\n    public void writeToBuf(BreezeBuffer breezeBuffer) throws BreezeException {\n        BreezeWriter.writeMessage(breezeBuffer, () -> {\n")
	for _, field := range fields {
		if field.Type.Number < core.Map {
			buf.WriteString("            " + javaTypes[field.Type.Number].breezeType)
		} else {
			buf.WriteString("            " + field.Name + "BreezeType")
		}
		buf.WriteString(".writeMessageField(breezeBuffer, " + strconv.Itoa(field.Index) + ", " + field.Name + ");\n")
	}
	buf.WriteString("        });\n    }\n\n")

	//readFrom
	buf.WriteString("    @Override\n    public Message readFromBuf(BreezeBuffer breezeBuffer) throws BreezeException {\n        BreezeReader.readMessage(breezeBuffer, (int breezeIndex) -> {\n            switch (breezeIndex) {\n")
	for _, field := range fields {
		buf.WriteString("                case " + strconv.Itoa(field.Index) + ":\n                    " + field.Name + " = ")
		if field.Type.Number < core.Map {
			buf.WriteString(javaTypes[field.Type.Number].breezeType)
		} else {
			buf.WriteString(field.Name + "BreezeType")
		}
		buf.WriteString(".read(breezeBuffer);\n                    break;\n")
	}
	buf.WriteString("                default: //skip unknown field\n                    BreezeReader.readObject(breezeBuffer, Object.class);\n            }\n        });\n        return this;\n    }\n\n")

	//interface methods
	buf.WriteString("    @Override\n    public String messageName() { return breezeSchema.getName(); }\n\n")
	buf.WriteString("    @Override\n    public String messageAlias() { return breezeSchema.getAlias(); }\n\n")
	buf.WriteString("    @Override\n    public Schema schema() { return breezeSchema; }\n\n")
	buf.WriteString("    @Override\n    public Message defaultInstance() { return new " + message.Name + "(); }\n\n")

	//setter and getter
	for _, field := range fields {
		buf.WriteString("    public " + jt.getTypeString(field.Type, false) + " get" + firstUpper(field.Name) + "() { return " + field.Name + "; }\n\n")
		buf.WriteString("    public " + message.Name + " set" + firstUpper(field.Name) + "(" + jt.getTypeString(field.Type, false) + " " + field.Name + ") { this." + field.Name + " = " + field.Name + "; return this;}\n\n")
	}
	buf.Truncate(buf.Len() - 1)
	buf.WriteString("}\n")

	return withPackageDirByName(message.Name, schema, pkg, false) + ".java", buf.Bytes(), nil
}

func (jt *JavaTemplate) getTypeImport(tp *core.Type, context *core.Context, tps []string) []string {
	switch tp.Number {
	case core.Array, core.Map: //only array or map value maybe contains message type
		tps = jt.getTypeImport(tp.ValueType, context, tps)
		tps = append(tps, "import java.util.*;\n") // need import collection
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

func (jt *JavaTemplate) generateService(schema *core.Schema, service *core.Service, context *core.Context, isImpl bool) (file string, content []byte, err error) {
	buf := &bytes.Buffer{}
	writeGenerateComment(buf, schema.Name)
	pkg := getJavaPkg(schema)
	buf.WriteString("package " + pkg + ";\n\n")
	//import
	importStr := make([]string, 0, 16)
	importStr = append(importStr, "import com.weibo.api.motan.rpc.ResponseFuture;\n")
	types := getAllParamType(service)
	if len(types) > 0 {
		for _, t := range types {
			importStr = jt.getTypeImport(t, context, importStr)
		}
	}
	importStr = sortUnique(importStr)
	for _, t := range importStr {
		buf.WriteString(t)
	}
	buf.WriteString("\n\n")

	//class body
	if isImpl {
		buf.WriteString("public class " + service.Name + "Impl implements " + service.Name + " {\n")
	} else {
		buf.WriteString("public interface " + service.Name + " {\n")
	}

	//methods
	for _, method := range sortMethods(service) {
		jt.writeMethod(method, buf, isImpl, false)
	}

	// async methods should not implement by server end, so let it placed after normal methods
	for _, method := range sortMethods(service) {
		jt.writeMethod(method, buf, isImpl, true)
	}
	buf.Truncate(buf.Len() - 1)
	buf.WriteString("}\n")
	fileName := service.Name
	if isImpl {
		fileName += "Impl"
	}
	return withPackageDirByName(fileName, schema, getJavaPkg(schema), false) + ".java", buf.Bytes(), nil
}

func (jt *JavaTemplate) writeMethod(method *core.Method, buf *bytes.Buffer, isImpl bool, async bool) {
	buf.WriteString("    ")
	if isImpl {
		buf.WriteString("@Override\n    public ")
	}

	if async {
		buf.WriteString("ResponseFuture")
	} else {
		if method.Return != nil {
			buf.WriteString(jt.getTypeString(method.Return, true))
		} else {
			buf.WriteString("void")
		}
	}
	name := method.Name
	if async {
		name += "Async"
	}
	buf.WriteString(" " + name + "(")
	if len(method.Params) > 0 {
		for i := 0; i < len(method.Params); i++ {
			param := method.Params[i]
			buf.WriteString(jt.getTypeString(param.Type, false))
			buf.WriteString(" " + param.Name)
			buf.WriteString(", ")
		}
		buf.Truncate(buf.Len() - 2)
	}
	if isImpl {
		buf.WriteString(") {\n")
		if async {
			buf.WriteString("        //This method is only used for client end asynchronous calls, should not implement it in serve end\n")
			buf.WriteString("        throw new RuntimeException(\"should not implement\");\n")
		} else {
			buf.WriteString("        //TODO implement this method\n")
			if method.Return != nil {
				buf.WriteString("        return null;\n")
			}
		}
		buf.WriteString("    }\n\n")
	} else {
		buf.WriteString(");\n\n")
	}
}

func getJavaPkg(schema *core.Schema) string {
	pkg := schema.Options[core.JavaPackage]
	if pkg == "" {
		pkg = schema.Package
	}
	return pkg
}

func getAllParamType(service *core.Service) []*core.Type {
	types := make([]*core.Type, 0, 16)
	for _, method := range service.Methods {
		if method.Return != nil {
			types = append(types, method.Return)
		}
		if len(method.Params) > 0 {
			for _, param := range method.Params {
				types = append(types, param.Type)
			}
		}
	}
	return types
}
