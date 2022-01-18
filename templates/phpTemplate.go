package templates

import (
	"bytes"
	"errors"
	"strconv"
	"strings"

	"github.com/weibreeze/breeze-generator/core"
)

var (
	phpTypes = map[int]*phpTypeInfo{
		core.Bool:    {useString: "use Breeze\\Types\\TypeBool;\n", descString: "TypeBool::instance()"},
		core.String:  {useString: "use Breeze\\Types\\TypeString;\n", descString: "TypeString::instance()"},
		core.Byte:    {useString: "use Breeze\\Types\\TypeByte;\n", descString: "TypeByte::instance()"},
		core.Bytes:   {useString: "use Breeze\\Types\\TypeBytes;\n", descString: "TypeBytes::instance()"},
		core.Int16:   {useString: "use Breeze\\Types\\TypeInt16;\n", descString: "TypeInt16::instance()"},
		core.Int32:   {useString: "use Breeze\\Types\\TypeInt32;\n", descString: "TypeInt32::instance()"},
		core.Int64:   {useString: "use Breeze\\Types\\TypeInt64;\n", descString: "TypeInt64::instance()"},
		core.Float32: {useString: "use Breeze\\Types\\TypeFloat32;\n", descString: "TypeFloat32::instance()"},
		core.Float64: {useString: "use Breeze\\Types\\TypeFloat64;\n", descString: "TypeFloat64::instance()"},
		core.Array:   {useString: "use Breeze\\Types\\TypePackedArray;\n", descString: "new TypePackedArray("},
		core.Map:     {useString: "use Breeze\\Types\\TypePackedMap;\n", descString: "new TypePackedMap("},
		core.Msg:     {useString: "use Breeze\\Types\\TypeMessage;\n", descString: "new TypeMessage(new "},
	}
)

type phpTypeInfo struct {
	useString  string
	descString string
}

//PHPTemplate : can generate php code according to schema
type PHPTemplate struct {
}

//Name : template name
func (pt *PHPTemplate) Name() string {
	return Php
}

//GenerateCode : generate php code
func (pt *PHPTemplate) GenerateCode(schema *core.Schema, context *core.Context) (contents map[string][]byte, err error) {
	contents = make(map[string][]byte)
	if len(schema.Messages) > 0 {
		for _, message := range schema.Messages {
			var file string
			var content []byte
			if message.IsEnum {
				file, content, err = pt.generateEnum(schema, message, context)
			} else {
				file, content, err = pt.generateMessage(schema, message, context)
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
			file, content, err := pt.generateService(schema, service, context)
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

func (pt *PHPTemplate) generateMessage(schema *core.Schema, message *core.Message, context *core.Context) (file string, content []byte, err error) {
	buf := &bytes.Buffer{}
	buf.WriteString("<?php\n")
	writeGenerateComment(buf, schema.Name)
	// fix : none package in breeze
	ns:= pt.getNamespace(schema.Package)
	if ns!=""{
		buf.WriteString("namespace " +ns+ ";\n\n")
	}
	buf.WriteString("use Breeze\\Breeze;\nuse Breeze\\BreezeException;\nuse Breeze\\BreezeReader;\nuse Breeze\\BreezeWriter;\nuse Breeze\\Buffer;\nuse Breeze\\FieldDesc;\nuse Breeze\\Message;\nuse Breeze\\Schema;\n")

	fields := sortFields(message)

	importStr := make([]string, 0, 16) //all type use string
	for _, field := range fields {
		importStr = pt.getTypeImport(field.Type, importStr)
	}
	if len(importStr) > 0 {
		importStr = sortUnique(importStr)
		for _, t := range importStr {
			buf.WriteString(t)
		}
	}

	//class body
	buf.WriteString("\nclass ")
	buf.WriteString(message.Name)
	buf.WriteString(" implements Message {\n")

	buf.WriteString("    private static $_schema;\n")
	buf.WriteString("    private static $_inited = false;\n")
	//fields type
	for _, field := range fields {
		buf.WriteString("    private static $_" + field.Name + "Type;\n")
	}
	buf.WriteString("\n")
	//fields
	for _, field := range fields {
		buf.WriteString("    private $" + field.Name + ";\n")
	}

	//construct
	buf.WriteString("\n    public function __construct() {\n        if (!self::$_inited) {\n")
	for _, field := range fields {
		desc, err := pt.getTypeString(field.Type)
		if err != nil {
			return "", nil, err
		}
		buf.WriteString("            self::$_" + field.Name + "Type = " + desc + ";\n")
	}
	buf.WriteString("            self::$_inited = true;\n        }\n    }\n\n")

	//initSchema
	buf.WriteString("    private function initSchema() {\n        self::$_schema = new Schema();\n        self::$_schema->setName('" + schema.OrgPackage + "." + message.Name + "');\n")
	if message.Alias != "" {
		buf.WriteString("        self::$_schema->setAlias('" + message.Alias + "');\n")
	}
	for _, field := range fields {
		buf.WriteString("        self::$_schema->putField(new FieldDesc(" + strconv.Itoa(field.Index) + ",'" + field.Name + "', self::$_" + field.Name + "Type));\n")
	}
	buf.WriteString("    }\n\n")

	//writeTo
	buf.WriteString("    public function writeTo(Buffer $buf) {\n        BreezeWriter::writeMessage($buf, function (Buffer $funcBuf) {\n")
	for _, field := range fields {
		buf.WriteString("            BreezeWriter::writeMessageField($funcBuf, " + strconv.Itoa(field.Index) + ", $this->" + field.Name + ", self::$_" + field.Name + "Type);\n")
	}
	buf.WriteString("        });\n    }\n\n")

	//readFrom
	buf.WriteString("    public function readFrom(Buffer $buf) {\n        BreezeReader::readMessage($buf, function (Buffer $funcBuf, $index) {\n            switch ($index) {\n")
	for _, field := range fields {
		buf.WriteString("                case " + strconv.Itoa(field.Index) + ":\n                    $this->" + field.Name + " = self::$_" + field.Name + "Type->read($funcBuf);\n                    break;\n")
	}
	buf.WriteString("                default: //skip unknown field\n                    BreezeReader::readValue($funcBuf);\n            }\n        });\n    }\n\n")

	//message interface methods
	pt.addCommonInterfaceMethod(buf, schema, message)

	//getter and setter
	for _, field := range fields {
		upperFieldName := firstUpper(field.Name)
		// int, float, bool对象未赋值时，get方法返回默认值，与其他语言对齐。
		if field.Type.Number == core.BoolType.Number {
			buf.WriteString("    public function get" + upperFieldName + "()\n    {\n        if (is_null($this->" + field.Name + ")) {\n            return false;\n        }\n        return $this->" + field.Name + "; }\n\n")
		} else if (field.Type.Number == core.Int16Type.Number) || (field.Type.Number == core.Int32Type.Number) || (field.Type.Number == core.Int64Type.Number) {
			buf.WriteString("    public function get" + upperFieldName + "()\n    {\n        if (is_null($this->" + field.Name + ")) {\n            return 0;\n        }\n        return $this->" + field.Name + "; }\n\n")
		} else if (field.Type.Number == core.Float32Type.Number) || (field.Type.Number == core.Float64Type.Number) {
			buf.WriteString("    public function get" + upperFieldName + "()\n    {\n        if (is_null($this->" + field.Name + ")) {\n            return 0.0;\n        }\n        return $this->" + field.Name + "; }\n\n")
		} else {
			buf.WriteString("    public function get" + upperFieldName + "() { return $this->" + field.Name + "; }\n\n")
		}
		buf.WriteString("    public function set" + upperFieldName + "($value) {\n        if (Breeze::$CHECK_VALUE && !self::$_" + field.Name + "Type->checkType($value)) {\n")
		buf.WriteString("            throw new BreezeException('check type fail. method:" + message.Name + "->set" + upperFieldName + "');\n        }\n")
		buf.WriteString("        $this->" + field.Name + " = $value;\n        return $this;\n    }\n\n")
	}
	buf.Truncate(buf.Len() - 1)

	//end of class
	buf.WriteString("}\n")
	return withPackageDir(message.Name, schema, context, true) + ".php", buf.Bytes(), nil
}

func (pt *PHPTemplate) generateEnum(schema *core.Schema, message *core.Message, context *core.Context) (file string, content []byte, err error) {
	buf := &bytes.Buffer{}
	buf.WriteString("<?php\n")
	writeGenerateComment(buf, schema.Name)
	buf.WriteString("namespace " + pt.getNamespace(schema.Package) + ";\n\n")
	buf.WriteString("use Breeze\\BreezeException;\nuse Breeze\\BreezeReader;\nuse Breeze\\BreezeWriter;\nuse Breeze\\Buffer;\nuse Breeze\\FieldDesc;\nuse Breeze\\Message;\nuse Breeze\\Schema;\nuse Breeze\\Types\\TypeInt32;\n")

	//class body
	buf.WriteString("\nclass ")
	buf.WriteString(message.Name)
	buf.WriteString(" implements Message {\n")

	// const
	enumValue := sortEnumValues(message)
	for _, value := range enumValue {
		buf.WriteString("    const " + value.Name + " = " + strconv.Itoa(value.Index) + ";\n")
	}

	//fields
	buf.WriteString("    private static $_schema;\n")
	buf.WriteString("    private $enumValue;\n")

	//construct
	buf.WriteString("\n    public function __construct($enumValue = 0) {\n")
	buf.WriteString("        if (!is_integer($enumValue)) {\n            throw new BreezeException('enum number must be integer');\n        }\n        $this->enumValue = $enumValue;\n    }\n\n")

	// enum value
	buf.WriteString("    public function value() {\n        return $this->enumValue;\n    }\n\n")

	//initSchema
	buf.WriteString("    private function initSchema() {\n        self::$_schema = new Schema();\n        self::$_schema->setName('" + schema.OrgPackage + "." + message.Name + "');\n")
	if message.Alias != "" {
		buf.WriteString("        self::$_schema->setAlias('" + message.Alias + "');\n")
	}
	buf.WriteString("        self::$_schema->putField(new FieldDesc(1, 'enumNumber', TypeInt32::instance()));\n    }\n\n")

	//writeTo
	buf.WriteString("    public function writeTo(Buffer $buf) {\n        BreezeWriter::writeMessage($buf, function (Buffer $funcBuf) {\n")
	buf.WriteString("            BreezeWriter::writeMessageField($funcBuf, 1, $this->enumValue, TypeInt32::instance());\n        });\n    }\n\n")

	//readFrom
	buf.WriteString("    public function readFrom(Buffer $buf) {\n        BreezeReader::readMessage($buf, function (Buffer $funcBuf, $index) {\n            switch ($index) {\n")
	buf.WriteString("                case 1:\n                    $number = TypeInt32::instance()->read($funcBuf);\n")
	buf.WriteString("                    switch ($number) {\n")
	for _, value := range enumValue {
		buf.WriteString("                        case " + strconv.Itoa(value.Index) + ":\n                            $this->enumValue = self::" + value.Name + ";\n                            break;\n")
	}
	buf.WriteString("                        default:\n                            throw new BreezeException('unknown enum number ' . $number);\n                    }\n                    break;\n")
	buf.WriteString("                default: // for compatibility\n                    BreezeReader::readValue($funcBuf);\n            }\n        });\n    }\n\n")

	//message interface methods
	pt.addCommonInterfaceMethod(buf, schema, message)

	//end of class
	buf.WriteString("}\n")
	return withPackageDir(message.Name, schema, context, true) + ".php", buf.Bytes(), nil
}

func (pt *PHPTemplate) getTypeImport(tp *core.Type, tps []string) []string {
	tps = append(tps, phpTypes[tp.Number].useString)
	switch tp.Number {
	case core.Array:
		tps = pt.getTypeImport(tp.ValueType, tps)
	case core.Map:
		tps = pt.getTypeImport(tp.KeyType, tps) //key type need import
		tps = pt.getTypeImport(tp.ValueType, tps)
	case core.Msg:
		if strings.Index(tp.Name, ".") > -1 { //not same package
			tps = append(tps, "use "+pt.getNamespace(tp.Name)+";\n")
		}
	}
	return tps
}

func (pt *PHPTemplate) getTypeString(tp *core.Type) (string, error) {
	desc := phpTypes[tp.Number].descString
	if desc == "" {
		return "", errors.New("can not find type desc. type:" + tp.TypeString)
	}
	switch tp.Number {
	case core.Array:
		vDesc, err := pt.getTypeString(tp.ValueType)
		if err != nil {
			return "", err
		}
		desc = desc + vDesc + ")"
	case core.Map:
		keyDesc, err := pt.getTypeString(tp.KeyType)
		if err != nil {
			return "", err
		}
		vDesc, err := pt.getTypeString(tp.ValueType)
		if err != nil {
			return "", err
		}
		desc = desc + keyDesc + ", " + vDesc + ")"
	case core.Msg:
		desc = desc + tp.Name[strings.LastIndex(tp.Name, ".")+1:] + "())"
	}
	return desc, nil
}

func (pt *PHPTemplate) addCommonInterfaceMethod(buf *bytes.Buffer, schema *core.Schema, message *core.Message) {
	buf.WriteString("    public function defaultInstance() { return new " + message.Name + "(); }\n\n")
	buf.WriteString("    public function messageName() { return '" + schema.OrgPackage + "." + message.Name + "'; }\n\n")
	buf.WriteString("    public function messageAlias() { return '" + message.Alias + "'; }\n\n")
	buf.WriteString("    public function schema() { \n        if (is_null(self::$_schema)) {\n            $this->initSchema();\n        }\n        return self::$_schema; }\n\n")
}

func (pt *PHPTemplate) generateService(schema *core.Schema, service *core.Service, context *core.Context) (file string, content []byte, err error) {
	//TODO implement
	return "", nil, nil
}

func (pt *PHPTemplate) generateMotanClient(schema *core.Schema, service *core.Service, context *core.Context) (file string, content []byte, err error) {
	//TODO implement
	return "", nil, nil
}

func (pt *PHPTemplate) getNamespace(pkg string) string {
	// fix: none package in breeze
	if pkg==""{
		return ""
	}
	return toCamelCase(pkg, "\\")
}
