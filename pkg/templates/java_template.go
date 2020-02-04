package templates

import (
	"bytes"
	"fmt"
	"github.com/weibreeze/breeze-generator/pkg/utils"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/weibreeze/breeze-generator/pkg/core"
)

const (
	OptionJavaMavenProject = "java_maven_project"
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

// PostAllGenerated: handler for all schema generated
func (jt *JavaTemplate) PostAllGenerated(context *core.Context) error {
	const mavenPomTemplate = `
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
		 xmlns="http://maven.apache.org/POM/4.0.0"
		 xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
	<modelVersion>4.0.0</modelVersion>
	<groupId>%s</groupId>
	<artifactId>%s</artifactId>
	<version>%s</version>

	<properties>
		<project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
		<breeze.version>0.1.3</breeze.version>
	</properties>

	<dependencies>
		<dependency>
			<groupId>com.weibo</groupId>
			<artifactId>breeze-core</artifactId>
			<version>${breeze.version}</version>
		</dependency>
	</dependencies>

	<build>
		<plugins>
			<plugin>
				<artifactId>maven-compiler-plugin</artifactId>
				<version>3.1</version>
				<configuration>
					<source>1.8</source>
					<target>1.8</target>
				</configuration>
			</plugin>
		</plugins>
	</build>
</project>`
	javaMavenProject := context.Options[OptionJavaMavenProject]
	if javaMavenProject == "" {
		// not configured to create maven project, just return
		return nil
	}
	groupAndArtifact := strings.Split(javaMavenProject, ":")
	if len(groupAndArtifact) != 2 {
		return fmt.Errorf("invaild maven project: %s", javaMavenProject)
	}
	version := context.Options[core.PackageVersion]
	if version == "" {
		return fmt.Errorf("no version specified")
	}

	mavenPom := fmt.Sprintf(mavenPomTemplate, groupAndArtifact[0], groupAndArtifact[1], version)
	// Maven project structure
	// maven_project
	//   |- pom.xml
	//   |- src
	//       |- main
	//           |- java
	//           |- resources
	generateRootPath := context.WritePath
	if generateRootPath[len(generateRootPath)-1:] != core.PathSeparator {
		generateRootPath += core.PathSeparator
	}
	generateRootPath = generateRootPath + jt.Name()
	mavenProjectDir := generateRootPath + core.PathSeparator + "maven_project"
	javaSrcPath := strings.Join([]string{mavenProjectDir, "src", "main", "java"}, core.PathSeparator)
	resourcesPath := strings.Join([]string{mavenProjectDir, "src", "main", "resources"}, core.PathSeparator)
	err := os.MkdirAll(javaSrcPath, core.DefaultNewDirectoryMode)
	if err != nil {
		return err
	}
	err = os.MkdirAll(resourcesPath, core.DefaultNewDirectoryMode)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(mavenProjectDir+core.PathSeparator+"pom.xml", []byte(mavenPom), core.DefaultNewRegularFileMode)
	if err != nil {
		return err
	}
	files, err := ioutil.ReadDir(generateRootPath)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.Name() == "maven_project" {
			continue
		}
		err = utils.Copy(generateRootPath+core.PathSeparator+f.Name(), javaSrcPath+core.PathSeparator+f.Name())
		if err != nil {
			return err
		}
	}
	return nil
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

func (jt *JavaTemplate) generateEnum(schema *core.Schema, message *core.Message, context *core.Context) (file string, content []byte, err error) {
	buf := &bytes.Buffer{}
	writeGenerateComment(buf, schema.Name)
	pkg := schema.Options[core.JavaPackage]
	if pkg == "" {
		pkg = schema.Package
	}
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
	buf.WriteString("        @Override\n        public void writeToBuf(" + message.Name + " obj, BreezeBuffer buffer) throws BreezeException {\n")
	buf.WriteString("            BreezeWriter.writeMessage(buffer, () -> {\n                TYPE_INT32.writeMessageField(buffer, 1, obj.number);\n            });\n        }\n\n")

	//readFrom
	buf.WriteString("        @Override\n        public " + message.Name + " readFromBuf(BreezeBuffer buffer) throws BreezeException {\n            int[] number = new int[]{-1};\n")
	buf.WriteString("            BreezeReader.readMessage(buffer, (int index) -> {\n                switch (index) {\n")
	buf.WriteString("                    case 1:\n                        number[0] = TYPE_INT32.read(buffer);\n                        break;")
	buf.WriteString("                    default:\n                        BreezeReader.readObject(buffer, Object.class);\n                }\n            });\n")
	buf.WriteString("            switch (number[0]) {\n")
	for _, value := range enumValues {
		buf.WriteString("                case " + strconv.Itoa(value.Index) + ":\n                   return " + value.Name + ";\n")
	}
	buf.WriteString("            }\n            throw new BreezeException(\"unknown enum number:\" + number[0]);\n        }\n\n")

	//interface methods
	buf.WriteString("        @Override\n        public String[] getNames() { return names; }\n    }\n}\n")
	return withJavaPackageDir(message.Name, schema) + ".java", buf.Bytes(), nil
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
	buf.WriteString("import com.weibo.breeze.*;\nimport com.weibo.breeze.message.Message;\nimport com.weibo.breeze.message.Schema;\nimport com.weibo.breeze.type.BreezeType;\n\n")

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
	buf.WriteString("import static com.weibo.breeze.Breeze.getBreezeType;\n")
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
	buf.WriteString("    @Override\n    public void writeToBuf(BreezeBuffer buffer) throws BreezeException {\n        BreezeWriter.writeMessage(buffer, () -> {\n")
	for _, field := range fields {
		if field.Type.Number < core.Map {
			buf.WriteString("            " + javaTypes[field.Type.Number].breezeType)
		} else {
			buf.WriteString("            " + field.Name + "BreezeType")
		}
		buf.WriteString(".writeMessageField(buffer, " + strconv.Itoa(field.Index) + ", " + field.Name + ");\n")
	}
	buf.WriteString("        });\n    }\n\n")

	//readFrom
	buf.WriteString("    @Override\n    public Message readFromBuf(BreezeBuffer buffer) throws BreezeException {\n        BreezeReader.readMessage(buffer, (int index) -> {\n            switch (index) {\n")
	for _, field := range fields {
		buf.WriteString("                case " + strconv.Itoa(field.Index) + ":\n                    " + field.Name + " = ")
		if field.Type.Number < core.Map {
			buf.WriteString(javaTypes[field.Type.Number].breezeType)
		} else {
			buf.WriteString(field.Name + "BreezeType")
		}
		buf.WriteString(".read(buffer);\n                    break;\n")
	}
	buf.WriteString("                default: //skip unknown field\n                    BreezeReader.readObject(buffer, Object.class);\n            }\n        });\n        return this;\n    }\n\n")

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

	return withJavaPackageDir(message.Name, schema) + ".java", buf.Bytes(), nil
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

func withJavaPackageDir(fileName string, schema *core.Schema) string {
	if schema.Options[core.WithPackageDir] != "" {
		return strings.ReplaceAll(schema.Options[core.JavaPackage], ".", string(os.PathSeparator)) + string(os.PathSeparator) + fileName
	}
	return fileName
}
