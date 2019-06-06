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
		core.Array:   {useString: "use Breeze\\Types\\TypeArray;\n", descString: "new TypeArray("},
		core.Map:     {useString: "use Breeze\\Types\\TypeMap;\n", descString: "new TypeMap("},
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
			file, content, err := pt.generateMessage(schema, message, context)
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
	buf.WriteString("namespace " + pt.getNamespace(schema.Package) + ";\n\n")
	buf.WriteString("use Breeze\\AbstractMessage;\nuse Breeze\\BreezeReader;\nuse Breeze\\BreezeWriter;\nuse Breeze\\Buffer;\nuse Breeze\\FieldDesc;\nuse Breeze\\MessageField;\nuse Breeze\\Schema;\n")

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
	buf.WriteString(" extends AbstractMessage {\n")

	//fields
	buf.WriteString("    private static $schema;\n")
	for _, field := range fields {
		buf.WriteString("    private $" + field.Name + ";\n")
	}

	//construct
	buf.WriteString("\n    public function __construct() {\n        if (is_null(self::$schema)) {\n            $this->initSchema();\n        }\n")
	for _, field := range fields {
		buf.WriteString("        $this->" + field.Name + " = new MessageField(self::$schema->getField(" + strconv.Itoa(field.Index) + "));\n")
	}
	buf.WriteString("    }\n\n")

	//initSchema
	buf.WriteString("    private function initSchema() {\n        self::$schema = new Schema();\n        self::$schema->setName('" + schema.Package + "." + message.Name + "');\n")
	if message.Alias != "" {
		buf.WriteString("        self::$schema->setAlias('" + message.Alias + "');\n")
	}
	for _, field := range fields {
		desc, err := pt.getTypeString(field.Type)
		if err != nil {
			return "", nil, err
		}
		buf.WriteString("        self::$schema->putField(new FieldDesc(" + strconv.Itoa(field.Index) + ",'" + field.Name + "', " + desc + "));\n")
	}
	buf.WriteString("    }\n\n")

	//writeTo
	buf.WriteString("    public function writeTo(Buffer $buf) {\n        BreezeWriter::writeMessage($buf, $this->getName(), function (Buffer $funcBuf) {\n            $this->writeFields($funcBuf")
	for _, field := range fields {
		buf.WriteString(", $this->" + field.Name)
	}
	buf.WriteString(");\n        });\n    }\n\n")

	//readFrom
	buf.WriteString("    public function readFrom(Buffer $buf) {\n        BreezeReader::readMessage($buf, function (Buffer $funcBuf, $index) {\n            switch ($index) {\n")
	for _, field := range fields {
		buf.WriteString("                case " + strconv.Itoa(field.Index) + ":\n                    return BreezeReader::readField($funcBuf, $this->" + field.Name + ");\n")
	}
	buf.WriteString("                default: //skip unknown field\n                    BreezeReader::readValue($funcBuf);\n            }\n        });\n    }\n\n")

	//message interface methods
	buf.WriteString("    public function defaultInstance() { return new " + message.Name + "(); }\n\n")
	buf.WriteString("    public function getName() { return self::$schema->getName(); }\n\n")
	buf.WriteString("    public function getAlias() { return self::$schema->getAlias(); }\n\n")
	buf.WriteString("    public function getSchema() { return self::$schema; }\n\n")

	//getter and setter
	for _, field := range fields {
		buf.WriteString("    public function get" + firstUpper(field.Name) + "() { return $this->" + field.Name + "->getValue(); }\n\n")
		buf.WriteString("    public function set" + firstUpper(field.Name) + "($value) {\n        $this->" + field.Name + "->setValue($value);\n        return $this;\n    }\n\n")
	}

	//end of class
	buf.WriteString("}\n")
	return withPackageDir(message.Name, schema) + ".php", buf.Bytes(), nil
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

func (pt *PHPTemplate) generateService(schema *core.Schema, service *core.Service, context *core.Context) (file string, content []byte, err error) {
	//TODO implement
	return "", nil, nil
}

func (pt *PHPTemplate) generateMotanClient(schema *core.Schema, service *core.Service, context *core.Context) (file string, content []byte, err error) {
	//TODO implement
	return "", nil, nil
}

func (pt *PHPTemplate) getNamespace(pkg string) string {
	items := strings.Split(pkg, ".")
	if len(items) == 0 {
		return ""
	}
	var ns string
	for _, item := range items {
		ns = ns + firstUpper(item) + "\\"
	}
	return ns[:len(ns)-1]
}
