package templates

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/weibreeze/breeze-generator/core"
)

var (
	goTypes = map[int]*goTypeInfo{
		core.Bool:    {typeString: "bool", writeTypeString: "breeze.WriteBool", readTypeString: "breeze.ReadBool"},
		core.String:  {typeString: "string", writeTypeString: "breeze.WriteString", readTypeString: "breeze.ReadString"},
		core.Byte:    {typeString: "byte", writeTypeString: "breeze.WriteByte", readTypeString: "breeze.ReadByte"},
		core.Bytes:   {typeString: "[]byte", writeTypeString: "breeze.WriteBytes", readTypeString: "breeze.ReadBytes"},
		core.Int16:   {typeString: "int16", writeTypeString: "breeze.WriteInt16", readTypeString: "breeze.ReadInt16"},
		core.Int32:   {typeString: "int32", writeTypeString: "breeze.WriteInt32", readTypeString: "breeze.ReadInt32"},
		core.Int64:   {typeString: "int64", writeTypeString: "breeze.WriteInt64", readTypeString: "breeze.ReadInt64"},
		core.Float32: {typeString: "float32", writeTypeString: "breeze.WriteFloat32", readTypeString: "breeze.ReadFloat32"},
		core.Float64: {typeString: "float64", writeTypeString: "breeze.WriteFloat64", readTypeString: "breeze.ReadFloat64"},
		core.Array:   {typeString: "[]"},
		core.Map:     {typeString: "map["},
	}
)

type goTypeInfo struct {
	typeString      string
	writeTypeString string
	readTypeString  string
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
	fileName = withPackageDir(fileName, schema, false)
	contents[fileName+".go"] = content.Bytes()
	return contents, nil
}

var (
	aliasImprotNameData = map[string]string{}
	hashCounter         = 0
)

func (gt *GoTemplate) getAliasImprotName(schema *core.Schema, importStr string, context *core.Context) string {
	hint := filepath.Base(filepath.Dir(importStr)) + "_" + filepath.Base(importStr)
	hash := ""
	for k, v := range aliasImprotNameData {
		if v == importStr {
			return fmt.Sprintf("p_%s_%s", k, hint)
		}
	}

	h := md5.New()
	h.Write([]byte(importStr))
	hash = fmt.Sprintf("%x", h.Sum(nil))[0:12]

	if v, ok := aliasImprotNameData[hash]; !ok {
		aliasImprotNameData[hash] = importStr
	} else {
		if v != importStr {
			hash += fmt.Sprintf("%d", hashCounter)
			hashCounter++
		}
	}

	return fmt.Sprintf("p_%s_%s", hash, hint)
}

func (gt *GoTemplate) getImportInfo(field *core.Field, importStrArr []string, context *core.Context, schema *core.Schema) (importStr []string, typeString string) {
	return gt.getImportInfoByType(field.Type, importStrArr, context, schema)
}

func (gt *GoTemplate) getImportInfoByType(tp *core.Type, importStrArr []string, context *core.Context, schema *core.Schema) (importStr []string, typeString string) {
	typeString = gt.getTypeString(tp)
	if strings.Contains(typeString, "*") {
		if len(importStrArr) > 0 {
			pkgPath := importStrArr[len(importStrArr)-1]
			if strings.Replace(pkgPath, "/", ".", -1) != schema.Package {
				aliasPkg := gt.getAliasImprotName(schema, pkgPath, context)
				pkgPath = aliasPkg + " " + pkgPath
				importStrArr[len(importStrArr)-1] = pkgPath
				if strings.Contains(typeString, ".") {
					prefix := typeString[:strings.Index(typeString, "*")]
					suffix := typeString[strings.LastIndex(typeString, ".")+1:]
					typeString = prefix + "*" + aliasPkg + "." + suffix
				}
			}
		} else {
			if strings.Contains(typeString, ".") {
				prefix := typeString[:strings.Index(typeString, "*")+1]
				suffix := typeString[strings.LastIndex(typeString, ".")+1:]
				typeString = prefix + suffix
			}
		}
	}
	if strings.HasPrefix(typeString, "**") {
		typeString = typeString[1:]
	} else if strings.HasPrefix(typeString, "*[]") {
		typeString = typeString[1:]
	} else if strings.HasPrefix(typeString, "*map") {
		typeString = typeString[1:]
	}
	return importStrArr, typeString
}
func (gt *GoTemplate) generateMessage(schema *core.Schema, message *core.Message, context *core.Context, buf *bytes.Buffer, importStr []string) ([]string, error) {
	buf.WriteString("type " + message.Name + " struct {\n")
	fields := sortFields(message) //sorted fields
	var tps []string
	for _, field := range fields {
		importStr0 := gt.getTypeImport(schema, field.Type, tps, context)
		importStr0, typeString := gt.getImportInfo(field, importStr0, context, schema)
		importStr = append(importStr, importStr0...)
		buf.WriteString("	" + firstUpper(field.Name) + " " + typeString + "\n")
	}
	buf.WriteString("}\n\n")

	//writeTo
	shortName := strings.ToLower(message.Name[:1])
	funcName := "func (" + shortName + " *" + message.Name + ")"
	buf.WriteString(funcName + " WriteTo(buf *breeze.Buffer) error {\n	return breeze.WriteMessageWithoutType(buf, func(buf *breeze.Buffer) {\n")
	for _, field := range fields {
		fieldName := shortName + "." + firstUpper(field.Name)
		params := "buf, " + strconv.Itoa(field.Index) + ", " + fieldName
		if field.Type.Number < core.Map {
			buf.WriteString("		" + goTypes[field.Type.Number].writeTypeString + "Field(" + params + ")\n")
		} else {
			switch field.Type.Number {
			case core.Array:
				buf.WriteString("		if len(" + fieldName + ") > 0 {\n")
				buf.WriteString("			breeze.WriteArrayField(buf, " + strconv.Itoa(field.Index) + ", len(" + fieldName + "), func(buf *breeze.Buffer) {\n")
				gt.writeArray(buf, field.Type, fieldName, 1)
				buf.WriteString("			})\n")
				buf.WriteString("		}\n")
			case core.Map:
				buf.WriteString("		if len(" + fieldName + ") > 0 {\n")
				buf.WriteString("			breeze.WriteMapField(buf, " + strconv.Itoa(field.Index) + ", len(" + fieldName + "), func(buf *breeze.Buffer) {\n")
				gt.writeMap(buf, field.Type, fieldName, 1)
				buf.WriteString("			})\n")
				buf.WriteString("		}\n")
			case core.Msg:
				buf.WriteString("		if " + fieldName + " != nil {\n			breeze.WriteMessageField(")
				buf.WriteString(params + ")\n		}\n")
			}
		}
	}
	buf.WriteString("	})\n}\n\n")

	//readFrom
	buf.WriteString(funcName + " ReadFrom(buf *breeze.Buffer) error {\n		return breeze.ReadMessageField(buf, func(buf *breeze.Buffer, index int) (err error) {\n		switch index {\n")
	for _, field := range fields {
		fieldName := shortName + "." + firstUpper(field.Name)
		buf.WriteString("		case " + strconv.Itoa(field.Index) + ":\n")
		tp := field.Type
		if field.Type.Number < core.Map {
			buf.WriteString("			err = " + goTypes[tp.Number].readTypeString + "(buf, &" + fieldName + ")\n")
		} else {
			switch field.Type.Number {
			case core.Array:
				gt.readArray(buf, tp, fieldName, 1, schema, context)
			case core.Map:
				gt.readMap(buf, tp, fieldName, 1, schema, context)
			case core.Msg:
				tpStr := gt.getTypeString(tp)[1:]
				if strings.Contains(tp.TypeString, ".") {
					s := strings.Replace(tp.TypeString, ".", "/", -1)
					importStr := filepath.Dir(s)
					basename := filepath.Base(s)
					isInSelfPackage := strings.Replace(importStr, "/", ".", -1) == schema.Package
					prefix := ""
					if context.Options != nil {
						prefix = context.Options[core.GoPackagePrefix]
						prefix = strings.TrimSuffix(prefix, "/") + "/"
						importStr = prefix + importStr
					}
					if !isInSelfPackage {
						tpStr = gt.getAliasImprotName(schema, importStr, context) + "." + basename
					} else {
						tpStr = basename
					}
				}
				if isEnum(field.Type, schema, context) {
					buf.WriteString("			var value " + tpStr + "\n			result, err := breeze.ReadByEnum(buf, value, true)\n			if err == nil {\n")
					buf.WriteString("				" + fieldName + " = result.(*" + tpStr + ")\n			}\n")
				} else {
					buf.WriteString("			" + fieldName + " = &" + tpStr + "{}\n")
					buf.WriteString("			return breeze.ReadByMessage(buf, " + fieldName + ")\n")
				}
			}
		}
	}
	buf.WriteString("		default: //skip unknown field\n			_, err = breeze.ReadValue(buf, nil)\n		}\n		return err\n	})\n}\n\n")

	//interface methods
	gt.addCommonInterfaceMethod(funcName, gt.schemaName(message.Name), buf)
	return importStr, nil
}

func (gt *GoTemplate) writeMap(buf *bytes.Buffer, tp *core.Type, name string, recursion int) {
	blank := "			"
	for i := 0; i < recursion; i++ {
		blank += "	"
	}
	recStr := strconv.Itoa(recursion)
	if tp.ValueType.Number < core.Map {
		if tp.KeyType.Number == core.String {
			switch tp.ValueType.Number {
			case core.String:
				buf.WriteString(blank + "breeze.WriteStringStringMapEntries(buf, " + name + ")\n")
				return
			case core.Int32:
				buf.WriteString(blank + "breeze.WriteStringInt32MapEntries(buf, " + name + ")\n")
				return
			case core.Int64:
				buf.WriteString(blank + "breeze.WriteStringInt64MapEntries(buf, " + name + ")\n")
				return
			}
		}
		buf.WriteString(blank + goTypes[tp.KeyType.Number].writeTypeString + "Type(buf)\n")
		buf.WriteString(blank + goTypes[tp.ValueType.Number].writeTypeString + "Type(buf)\n")
		buf.WriteString(blank + "for k" + recStr + ", v" + recStr + " := range " + name + " {")
		buf.WriteString(blank + "	" + goTypes[tp.KeyType.Number].writeTypeString + "(buf, k" + recStr + ", false)\n")
		buf.WriteString(blank + "	" + goTypes[tp.ValueType.Number].writeTypeString + "(buf, v" + recStr + ", false)\n")
		buf.WriteString(blank + "}\n")
		return
	}
	switch tp.ValueType.Number {
	case core.Map:
		buf.WriteString(blank + goTypes[tp.KeyType.Number].writeTypeString + "Type(buf)\n")
		buf.WriteString(blank + "breeze.WritePackedMapType(buf)\n")
		buf.WriteString(blank + "for k" + recStr + ", v" + recStr + " := range " + name + " {")
		buf.WriteString(blank + "	" + goTypes[tp.KeyType.Number].writeTypeString + "(buf, k" + recStr + ", false)\n")
		buf.WriteString(blank + "	breeze.WritePackedMap(buf, false, len(v" + recStr + "), func(buf *breeze.Buffer) {\n")
		gt.writeMap(buf, tp.ValueType, "v"+recStr, recursion+1)
		buf.WriteString(blank + "	})\n" + blank + "}\n")
	case core.Array:
		buf.WriteString(blank + goTypes[tp.KeyType.Number].writeTypeString + "Type(buf)\n")
		buf.WriteString(blank + "breeze.WritePackedArrayType(buf)\n")
		buf.WriteString(blank + "for k" + recStr + ", v" + recStr + " := range " + name + " {")
		buf.WriteString(blank + "	" + goTypes[tp.KeyType.Number].writeTypeString + "(buf, k" + recStr + ", false)\n")
		buf.WriteString(blank + "	breeze.WritePackedArray(buf, false, len(v" + recStr + "), func(buf *breeze.Buffer) {\n")
		gt.writeArray(buf, tp.ValueType, "v"+recStr, recursion+1)
		buf.WriteString(blank + "	})\n" + blank + "}\n")
	case core.Msg:
		buf.WriteString(blank + "first := true\n")
		buf.WriteString(blank + "for k" + recStr + ", v" + recStr + " := range " + name + " {\n")
		buf.WriteString(blank + "	if first {\n")
		buf.WriteString(blank + "		" + goTypes[tp.KeyType.Number].writeTypeString + "Type(buf)\n")
		buf.WriteString(blank + "		breeze.WriteMessageType(buf, v" + recStr + ".GetName())\n")
		buf.WriteString(blank + "		first = false\n	}\n")
		buf.WriteString(blank + "	" + goTypes[tp.KeyType.Number].writeTypeString + "(buf, k" + recStr + ", false)\n")
		buf.WriteString(blank + "	v" + recStr + ".WriteTo(buf)\n")
		buf.WriteString(blank + "}\n")
	}
}

func (gt *GoTemplate) writeArray(buf *bytes.Buffer, tp *core.Type, name string, recursion int) {
	blank := "			"
	for i := 0; i < recursion; i++ {
		blank += "	"
	}
	recStr := strconv.Itoa(recursion)
	if tp.ValueType.Number < core.Map {
		switch tp.ValueType.Number {
		case core.String:
			buf.WriteString(blank + "breeze.WriteStringArrayElems(buf, " + name + ")\n")
			return
		case core.Int32:
			buf.WriteString(blank + "breeze.WriteInt32ArrayElems(buf, " + name + ")\n")
			return
		case core.Int64:
			buf.WriteString(blank + "breeze.WriteInt64ArrayElems(buf, " + name + ")\n")
			return
		}
		buf.WriteString(blank + goTypes[tp.ValueType.Number].writeTypeString + "Type(buf)\n")
		buf.WriteString(blank + "for _, v" + recStr + " := range " + name + " {")
		buf.WriteString(blank + "	" + goTypes[tp.ValueType.Number].writeTypeString + "(buf, v" + recStr + ", false)\n")
		buf.WriteString(blank + "}\n")
		return
	}
	switch tp.ValueType.Number {
	case core.Map:
		buf.WriteString(blank + "breeze.WritePackedMapType(buf)\n")
		buf.WriteString(blank + "for _, v" + recStr + " := range " + name + " {")
		buf.WriteString(blank + "	breeze.WritePackedMap(buf, false, len(v" + recStr + "), func(buf *breeze.Buffer) {\n")
		gt.writeMap(buf, tp.ValueType, "v"+recStr, recursion+1)
		buf.WriteString(blank + "	})\n" + blank + "}\n")
	case core.Array:
		buf.WriteString(blank + "breeze.WritePackedArrayType(buf)\n")
		buf.WriteString(blank + "for _, v" + recStr + " := range " + name + " {")
		buf.WriteString(blank + "	breeze.WritePackedArray(buf, false, len(v" + recStr + "), func(buf *breeze.Buffer) {\n")
		gt.writeArray(buf, tp.ValueType, "v"+recStr, recursion+1)
		buf.WriteString(blank + "	})\n" + blank + "}\n")
	case core.Msg:
		buf.WriteString(blank + "first := true\n")
		buf.WriteString(blank + "for _, v" + recStr + " := range " + name + " {\n")
		buf.WriteString(blank + "	if first {\n")
		buf.WriteString(blank + "		breeze.WriteMessageType(buf, v" + recStr + ".GetName())\n")
		buf.WriteString(blank + "		first = false\n	}\n")
		buf.WriteString(blank + "	v" + recStr + ".WriteTo(buf)\n")
		buf.WriteString(blank + "}\n")
	}
}

func (gt *GoTemplate) readMap(buf *bytes.Buffer, tp *core.Type, name string, recursion int, schema *core.Schema, context *core.Context) {
	blank := "			"
	for i := 1; i < recursion; i++ {
		blank += "	"
	}
	withType := "false"
	assign := ":="
	if recursion == 1 {
		withType = "true"
		assign = "="
	}
	//direct map
	if tp.KeyType.Number == core.String && tp.ValueType.Number < core.Map {
		switch tp.ValueType.Number {
		case core.String:
			buf.WriteString(blank + name + ", err " + assign + " breeze.ReadStringStringMap(buf, " + withType + ")\n")
			return
		case core.Int32:
			buf.WriteString(blank + name + ", err " + assign + " breeze.ReadStringInt32Map(buf, " + withType + ")\n")
			return
		case core.Int64:
			buf.WriteString(blank + name + ", err " + assign + " breeze.ReadStringInt64Map(buf, " + withType + ")\n")
			return
		}
	}
	recStr := strconv.Itoa(recursion)
	buf.WriteString(blank + "size, err := breeze.ReadPackedSize(buf, " + withType + ")\n" + blank + "if err != nil {\n" + blank + "	return err\n" + blank + "}\n")
	tpStr := gt.getTypeString(tp)
	if strings.Contains(tpStr, ".") {
		tps := []string{}
		importStr0 := gt.getTypeImport(schema, tp, tps, context)
		importStr0, typeString0 := gt.getImportInfoByType(tp, importStr0, context, schema)
		tpStr = typeString0
	}
	buf.WriteString(blank + name + " " + assign + " make(" + tpStr + ", size)\n")
	buf.WriteString(blank + "err = breeze.ReadPacked(buf, size, true, func(buf *breeze.Buffer) error {\n")
	//read key
	buf.WriteString(blank + "	k" + recStr + ", err := " + goTypes[tp.KeyType.Number].readTypeString + "WithoutType(buf)\n")
	buf.WriteString(blank + "	if err != nil {\n" + blank + "		return err\n" + blank + "	}\n")

	//read value
	vname := "v" + recStr
	if tp.ValueType.Number < core.Map {
		buf.WriteString(blank + "	" + vname + ", err := " + goTypes[tp.ValueType.Number].readTypeString + "WithoutType(buf)\n")
	} else {
		switch tp.ValueType.Number {
		case core.Map:
			gt.readMap(buf, tp.ValueType, vname, recursion+1, schema, context)
		case core.Array:
			gt.readArray(buf, tp.ValueType, vname, recursion+1, schema, context)
		case core.Msg:
			tpStr := tpStr[strings.Index(tpStr, "*")+1:]
			if isEnum(tp.ValueType, schema, context) {
				buf.WriteString(blank + "	var enum " + tpStr + "\n")
				buf.WriteString(blank + "	result, err := enum.ReadEnum(buf, true)\n")
				vname = "result.(*" + tpStr + ")"
			} else {
				buf.WriteString(blank + "	" + vname + " := &" + tpStr + "{}\n")
				buf.WriteString(blank + "	err = " + vname + ".ReadFrom(buf)\n")
			}
		}
	}
	buf.WriteString(blank + "	if err == nil {\n" + blank + "		" + name + "[k" + recStr + "] = " + vname + "\n")
	buf.WriteString(blank + "	}\n" + blank + "	return err\n" + blank + "})\n")
	if recursion == 1 {
		buf.WriteString(blank + "return err\n")
	}
}

func (gt *GoTemplate) readArray(buf *bytes.Buffer, tp *core.Type, name string, recursion int, schema *core.Schema, context *core.Context) {
	blank := "			"
	for i := 1; i < recursion; i++ {
		blank += "	"
	}
	withType := "false"
	assign := ":="
	if recursion == 1 {
		withType = "true"
		assign = "="
	}
	//direct array
	switch tp.ValueType.Number {
	case core.String:
		buf.WriteString(blank + name + ", err " + assign + " breeze.ReadStringArray(buf, " + withType + ")\n")
		return
	case core.Int32:
		buf.WriteString(blank + name + ", err " + assign + " breeze.ReadInt32Array(buf, " + withType + ")\n")
		return
	case core.Int64:
		buf.WriteString(blank + name + ", err " + assign + " breeze.ReadInt64Array(buf, " + withType + ")\n")
		return
	}

	recStr := strconv.Itoa(recursion)
	buf.WriteString(blank + "size, err := breeze.ReadPackedSize(buf, " + withType + ")\n" + blank + "if err != nil {\n" + blank + "	return err\n" + blank + "}\n")

	tpStr := gt.getTypeString(tp)
	if strings.Contains(tpStr, ".") {
		tps := []string{}
		importStr0 := gt.getTypeImport(schema, tp, tps, context)
		importStr0, typeString0 := gt.getImportInfoByType(tp, importStr0, context, schema)
		tpStr = typeString0
	}
	buf.WriteString(blank + name + " " + assign + " make(" + tpStr + ", 0, size)\n")
	buf.WriteString(blank + "err = breeze.ReadPacked(buf, size, false, func(buf *breeze.Buffer) error {\n")

	//read value
	vname := "v" + recStr
	if tp.ValueType.Number < core.Map {
		buf.WriteString(blank + "	" + vname + ", err := " + goTypes[tp.ValueType.Number].readTypeString + "WithoutType(buf)\n")
	} else {
		switch tp.ValueType.Number {
		case core.Map:
			gt.readMap(buf, tp.ValueType, vname, recursion+1, schema, context)
		case core.Array:
			gt.readArray(buf, tp.ValueType, vname, recursion+1, schema, context)
		case core.Msg:
			tpStr := tpStr[strings.Index(tpStr, "*")+1:]
			if isEnum(tp.ValueType, schema, context) {
				buf.WriteString(blank + "	var enum " + tpStr + "\n")
				buf.WriteString(blank + "	result, err := enum.ReadEnum(buf, true)\n")
				vname = "result.(*" + tpStr + ")"
			} else {
				buf.WriteString(blank + "	" + vname + " := &" + tpStr + "{}\n")
				buf.WriteString(blank + "	err = " + vname + ".ReadFrom(buf)\n")
			}
		}
	}
	buf.WriteString(blank + "	if err == nil {\n" + blank + "		" + name + " = append(" + name + ", " + vname + ")\n")
	buf.WriteString(blank + "	}\n" + blank + "	return err\n" + blank + "})\n")
	if recursion == 1 {
		buf.WriteString(blank + "return err\n")
	}
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
	buf.WriteString(funcName + " WriteTo(buf *breeze.Buffer) error {\n	return breeze.WriteMessageWithoutType(buf, func(buf *breeze.Buffer) {\n")
	buf.WriteString("		breeze.WriteInt32Field(buf, 1, int32(" + shortName + "))\n	})\n}\n\n")

	// read from
	buf.WriteString(funcName + " ReadFrom(buf *breeze.Buffer) error {\n	return errors.New(\"can not read enum by Message.ReadFrom, Enum.ReadEnum is expected. name:\" + " + shortName + ".GetName())\n}\n\n")

	// read enum
	buf.WriteString(funcName + " ReadEnum(buf *breeze.Buffer, asAddr bool) (breeze.Enum, error) {\n	var number int32\n	e := breeze.ReadMessageField(buf, func(buf *breeze.Buffer, index int) (err error) {\n")
	buf.WriteString("		switch index {\n		case 1:\n			err = breeze.ReadInt32(buf, &number)\n")
	buf.WriteString("		default: //skip unknown field\n			_, err = breeze.ReadValue(buf, nil)\n		}\n		return err\n	})\n")
	buf.WriteString("	if e == nil {\n		var result " + message.Name + "\n		switch number {\n")
	for _, v := range fields {
		buf.WriteString("		case " + strconv.Itoa(v.Index) + ":\n			result = " + message.Name + firstUpper(v.Name) + "\n")
	}
	buf.WriteString("		default:\n			return nil, errors.New(\"unknown enum number \" + strconv.Itoa(int(number)))\n		}\n		if asAddr {\n			return &result, nil\n		}\n		return result, nil\n	}\n	return nil, e\n}\n\n")

	gt.addCommonInterfaceMethod(funcName, gt.schemaName(message.Name), buf)
	return importStr, nil
}

func (gt *GoTemplate) getTypeImport(schema *core.Schema, tp *core.Type, tps []string, context *core.Context) []string {
	switch tp.Number {
	case core.Array, core.Map: //only array or map value maybe contains message type
		tps = gt.getTypeImport(schema, tp.ValueType, tps, context)
	case core.Msg:
		index := strings.LastIndex(tp.Name, ".")
		if index > -1 && tp.Name[:index] != schema.Package { //not same package
			prefix := ""
			if context.Options != nil {
				prefix = context.Options[core.GoPackagePrefix]
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
		if strings.Contains(value, " ") {
			buf.WriteString("	" + strings.Replace(value, " ", " \"", 1) + "\"\n")
		} else {
			buf.WriteString("	\"" + value + "\"\n")
		}
	}
	buf.WriteString(")\n\n")
}
