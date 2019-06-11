package templates

import (
	"bytes"
	"sort"
	"strconv"
	"strings"

	"github.com/weibreeze/breeze-generator/core"
)

var (
	cppTypes = map[int]*cppTypeInfo{
		core.Bool:    {typeString: "bool"},
		core.String:  {typeString: "std::string"},
		core.Byte:    {typeString: "uint8_t"},
		core.Bytes:   {typeString: "std::vector<uint8_t>"},
		core.Int16:   {typeString: "int16_t"},
		core.Int32:   {typeString: "int32_t"},
		core.Int64:   {typeString: "int64_t"},
		core.Float32: {typeString: "float_t"},
		core.Float64: {typeString: "double_t"},
		core.Array:   {typeString: "std::vector<"},
		core.Map:     {typeString: "std::unordered_map<"},
	}
)

type cppTypeInfo struct {
	typeString string
}

type CppTemplate struct{}

func (ct *CppTemplate) Name() string {
	return Cpp
}

func (ct *CppTemplate) GenerateCode(schema *core.Schema, context *core.Context) (contents map[string][]byte, err error) {
	headerBuf := &bytes.Buffer{}
	contents = make(map[string][]byte)
	if err = ct.generateHeader(schema, headerBuf); err != nil {
		return nil, err
	}
	contents[schema.Name+".h"] = headerBuf.Bytes()
	cppBuf := &bytes.Buffer{}
	if err = ct.generateCpp(schema, cppBuf); err != nil {
		return nil, err
	}
	contents[schema.Name+".cpp"] = cppBuf.Bytes()
	return contents, nil
}

func (ct *CppTemplate) generateHeader(schema *core.Schema, buf *bytes.Buffer) error {
	if len(schema.Messages) > 0 {
		writeGenerateComment(buf, schema.Name)
		defineName := strings.ToUpper(strings.ReplaceAll(schema.Name, ".", "_"))
		buf.WriteString(
			"\n#ifndef BREEZE_CPP_" + defineName + "_H\n" +
				"#define BREEZE_CPP_" + defineName + "_H\n\n" +
				"#include \"serialize/breeze.h\"\n\n")
		var ml []*core.Message
		for _, message := range schema.Messages {
			ml = append(ml, message)
		}
		sort.Sort(MessageList(ml))
		for _, message := range ml {
			if err := ct.generateHeaderClass(message, buf); err != nil {
				return err
			}
		}
		buf.WriteString("#endif //BREEZE_CPP_" + defineName + "_H\n")
	}
	return nil
}

func (ct *CppTemplate) generateHeaderClass(message *core.Message, buf *bytes.Buffer) error {
	if message.IsEnum {
		buf.WriteString("class " + message.Name + " : public BreezeEnum {\n")
	} else {
		buf.WriteString("class " + message.Name + " : public BreezeMessage {\n")
	}
	buf.WriteString("public:\n")
	if message.IsEnum {
		buf.WriteString("	enum E" + message.Name + " {\n")
		fields := sortEnumValues(message)
		for _, v := range fields {
			buf.WriteString("		" + v.Name + " = " + strconv.Itoa(v.Index) + ",\n")
		}
		buf.WriteString("	};\n\n" +
			"	E" + message.Name + " value_{};\n\n")
	}

	buf.WriteString("	" + message.Name + "();\n\n")
	if message.IsEnum {
		buf.WriteString("	explicit " + message.Name + "(const E" + message.Name + " &e);\n\n" +
			"	friend bool operator==(const " + message.Name + " &a, const " + message.Name + " &b);\n\n" +
			"	friend bool operator==(const " + message.Name + " &a, const E" + message.Name + " &b);\n\n" +
			"	friend bool operator==(const E" + message.Name + " &a, const " + message.Name + " &b);\n\n" +
			"	friend bool operator!=(const " + message.Name + " &a, const " + message.Name + " &b);\n\n" +
			"	friend bool operator!=(const " + message.Name + " &a, const E" + message.Name + " &b);\n\n" +
			"	friend bool operator!=(const E" + message.Name + " &a, const " + message.Name + " &b);\n\n" +
			"	" + message.Name + " &operator=(const E" + message.Name + " &a);\n\n")
	}

	fields := sortFields(message)
	for _, field := range fields {
		buf.WriteString("	" + ct.getTypeString(field.Type) + " " + field.Name + "{};\n\n")
	}
	buf.WriteString(
		"	int write_to(BytesBuffer *buf) const override;\n\n" +
			"	int read_from(BytesBuffer *buf) override;\n\n" +
			"	std::string get_name() const override;\n\n" +
			"	std::string get_alias() override;\n\n" +
			"	std::shared_ptr<BreezeSchema> get_schema() override;\n\n" +
			"	void set_name(const std::string &name) override;\n\n")
	if message.IsEnum {
		buf.WriteString("	int read_enum(BytesBuffer *) override;\n\n")
	}
	buf.WriteString(
		"private:\n" +
			"	std::shared_ptr<BreezeSchema> schema_{};\n")
	buf.WriteString("};\n\n")
	return nil
}

func (ct *CppTemplate) generateCpp(schema *core.Schema, buf *bytes.Buffer) error {
	if len(schema.Messages) > 0 {
		writeGenerateComment(buf, schema.Name)
		buf.WriteString("\n#include \"serialize/" + schema.Name + ".h\"\n\n")
		for _, message := range schema.Messages {
			ct.generateMethodConstructor(schema, message, buf)
			ct.generateMethodWriteTo(message, buf)
			ct.generateMethodReadFrom(message, buf)
			buf.WriteString(
				"std::string " + message.Name + "::get_name() const { return schema_->name_; }\n\n" +
					"std::string " + message.Name + "::get_alias() { return schema_->alias_; }\n\n" +
					"std::shared_ptr<BreezeSchema> " + message.Name + "::get_schema() { return schema_; }\n\n" +
					"void " + message.Name + "::set_name(const std::string &name) {}\n\n")
		}
	}
	return nil
}

func (ct *CppTemplate) generateMethodConstructor(schema *core.Schema, message *core.Message, buf *bytes.Buffer) {
	buf.WriteString(message.Name + "::" + message.Name + "() {\n" +
		"	schema_ = std::make_shared<BreezeSchema>(BreezeSchema{});\n" +
		"	schema_->name_ = \"" + schema.Package + "." + message.Name + "\";\n")
	for _, field := range sortFields(message) {
		buf.WriteString("	schema_->put_field(std::make_shared<BreezeField>(BreezeField{" + strconv.Itoa(field.Index) + ", \"" + field.Name + "\", \"" + field.Type.TypeString + "\"}));\n")
	}
	buf.WriteString("}\n\n")

	if message.IsEnum {
		buf.WriteString(message.Name + "::" + message.Name + "(const E" + message.Name + " &e) : value_(e) {\n" +
			"	schema_ = std::make_shared<BreezeSchema>(BreezeSchema{});\n" +
			"	schema_->name_ = \"" + schema.Package + "." + message.Name + "\";\n" +
			"}\n\n" +
			"bool operator==(const " + message.Name + " &a, const " + message.Name + " &b) { return a.value_ == b.value_; }\n\n" +
			"bool operator==(const " + message.Name + " &a, const MyEnum::E" + message.Name + " &b) { return a.value_ == b; }\n\n" +
			"bool operator==(const MyEnum::E" + message.Name + " &a, const " + message.Name + " &b) { return a == b.value_; }\n\n" +
			"bool operator!=(const " + message.Name + " &a, const " + message.Name + " &b) { return a.value_ != b.value_; }\n\n" +
			"bool operator!=(const " + message.Name + " &a, const MyEnum::E" + message.Name + " &b) { return a.value_ != b; }\n\n" +
			"bool operator!=(const MyEnum::E" + message.Name + " &a, const " + message.Name + " &b) { return a != b.value_; }\n\n" +
			message.Name + " &" + message.Name + "::operator=(const " + message.Name + "::E" + message.Name + " &a) {\n" +
			"    this->value_ = a;\n" +
			"    return *this;\n" +
			"}\n\n")
	}
}

func (ct *CppTemplate) generateMethodWriteTo(message *core.Message, buf *bytes.Buffer) {
	buf.WriteString("int " + message.Name + "::write_to(BytesBuffer *buf) const {\n")
	buf.WriteString("	return breeze::write_message(buf, schema_->name_, [this, buf]() {\n")
	if message.IsEnum {
		buf.WriteString("		breeze::write_message_field(buf, 1, int32_t(this->value_));\n")
	} else {
		fields := sortFields(message)
		for _, field := range fields {
			buf.WriteString("		breeze::write_message_field(buf, " + strconv.Itoa(field.Index) + ", this->" + field.Name + ");" + "\n")
		}
	}
	buf.WriteString("	});\n")
	buf.WriteString("}\n\n")
}

func (ct *CppTemplate) generateMethodReadFrom(message *core.Message, buf *bytes.Buffer) {
	buf.WriteString("int " + message.Name + "::read_from(BytesBuffer *buf) {\n")
	if message.IsEnum {
		buf.WriteString("	return -1;\n")
	} else {
		buf.WriteString("	return breeze::read_message_by_field(buf, [this](BytesBuffer *buf, int index) {\n" +
			"		switch (index) {\n")
		fields := sortFields(message)
		for _, field := range fields {
			buf.WriteString("			case " + strconv.Itoa(field.Index) + ":\n" +
				"				return breeze::read_value(buf, this->" + field.Name + ");\n")
		}
		buf.WriteString("			default:\n" +
			"				return -1;\n" +
			"		}\n" +
			"	});\n")
	}
	buf.WriteString("}\n\n")
	if message.IsEnum {
		buf.WriteString("int " + message.Name + "::read_enum(BytesBuffer *buf) {\n" +
			"	int number;\n" +
			"	auto err = breeze::read_message_by_field(buf, [this, &number](BytesBuffer *buf, int index) {\n" +
			"		if (index == 1) {\n" +
			"			return breeze::read_value(buf, number);\n" +
			"		} else {\n" +
			"			return -1;\n" +
			"		}\n" +
			"	});\n" +
			"	if (err != 0) {\n" +
			"		return err;\n" +
			"	}\n" +
			"	switch (number) {\n")
		fields := sortEnumValues(message)
		for _, v := range fields {
			buf.WriteString("		case " + strconv.Itoa(v.Index) + ":\n" +
				"			this->value_ = " + v.Name + ";\n" +
				"			break;\n")
		}
		buf.WriteString("		default:\n" +
			"			return -1;\n" +
			"	}\n" +
			"	return 0;\n" +
			"}\n\n")
	}
}

func (ct *CppTemplate) getTypeString(tp *core.Type) string {
	if tp.Number < core.Map {
		return cppTypes[tp.Number].typeString
	}
	switch tp.Number {
	case core.Array:
		return cppTypes[tp.Number].typeString + ct.getTypeString(tp.ValueType) + ">"
	case core.Map:
		return cppTypes[tp.Number].typeString + ct.getTypeString(tp.KeyType) + ", " + ct.getTypeString(tp.ValueType) + ">"
	case core.Msg:
		return tp.Name
	}
	return ""
}

type MessageList []*core.Message

func (s MessageList) Len() int {
	return len(s)
}

func (s MessageList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s MessageList) Less(i, j int) bool {
	for _, field := range s[i].Fields {
		if strings.Contains(field.Type.TypeString, s[j].Name) {
			return false
		}
	}
	return true
}
