package templates

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"github.com/weibreeze/breeze-generator/core"
)

var (
	luaTypes = map[int]*luaTypeInfo{
		core.Bool:    {schemaTypeString: "bool", defaultValue: ""},
		core.String:  {schemaTypeString: "string", defaultValue: " or \"\""},
		core.Byte:    {schemaTypeString: "byte", defaultValue: " or nil"},
		core.Bytes:   {schemaTypeString: "bytes", defaultValue: " or nil"},
		core.Int16:   {schemaTypeString: "int16", defaultValue: " or 0"},
		core.Int32:   {schemaTypeString: "int32", defaultValue: " or 0"},
		core.Int64:   {schemaTypeString: "int64", defaultValue: " or 0"},
		core.Float32: {schemaTypeString: "float32", defaultValue: " or 0"},
		core.Float64: {schemaTypeString: "float64", defaultValue: " or 0"},
		core.Array:   {schemaTypeString: "packed_array", defaultValue: " or {}"},
		core.Map:     {schemaTypeString: "packed_map", defaultValue: " or {}"},
		core.Msg:     {schemaTypeString: "message", defaultValue: " or {}"},
	}
)

type luaTypeInfo struct {
	schemaTypeString string
	defaultValue     string
}

const (
	// LuaTemplateVersion as a lua code version
	LuaTemplateVersion = "0.0.1"
)

//LuaTemplate : can generate lua code according to schema
type LuaTemplate struct {
}

//Name : template name
func (lt *LuaTemplate) Name() string {
	return Lua
}

//GenerateCode : generate lua code
func (lt *LuaTemplate) GenerateCode(schema *core.Schema, context *core.Context) (contents map[string][]byte, err error) {
	contents = make(map[string][]byte)
	if len(schema.Messages) > 0 {
		for _, message := range schema.Messages {
			var file string
			var content []byte
			if message.IsEnum {
				file, content, err = lt.generateEnum(schema, message, context)
			} else {
				file, content, err = lt.generateMessage(schema, message, context)
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
			file, content, err := lt.generateService(schema, service, context)
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

func getWriteFieldString(field *core.Field) (res string) {
	switch field.Type.Number {
	case core.Array:
		res = `
        local ` + field.Name + `_size = #self.` + field.Name + `
        if ` + field.Name + `_size > 0 then
            brz_w.write_array_field(fbuf, ` + strconv.Itoa(field.Index) + `, ` + field.Name + `_size, function(fbuf)
                brz_w.write_` + field.Type.ValueType.TypeString + `_array_elems(fbuf, self.` + field.Name + `)
            end)
        end
`
	case core.Map:
		res = `
        local ` + field.Name + `_size = brz_tools.arr_size(self.` + field.Name + `)
        if ` + field.Name + `_size > 0 then
            brz_w.write_map_field(fbuf, 9, ` + field.Name + `_size, function(fbuf)
                brz_w.write_` + luaTypes[field.Type.KeyType.Number].schemaTypeString + `_type(fbuf)
                brz_w.write_` + luaTypes[field.Type.ValueType.Number].schemaTypeString + `_type(fbuf)
                for k,v in pairs(self.` + field.Name + `) do
                    brz_w.write_` + luaTypes[field.Type.KeyType.Number].schemaTypeString + `(fbuf, k, false)
                    brz_w.write_` + luaTypes[field.Type.ValueType.Number].schemaTypeString + `(fbuf, false, #v, function(fbuf)
                        v:write_to(fbuf)
                    end)
                end
            end)
        end
`
	default:
		res = "        brz_w.write_" + field.Type.TypeString + "_field(fbuf, " + strconv.Itoa(field.Index) + ", self." + field.Name + ")\n"
	}
	return
}

func (lt *LuaTemplate) generateMessage(schema *core.Schema, message *core.Message, context *core.Context) (file string, content []byte, err error) {
	buf := &bytes.Buffer{}
	buf.WriteString(`-- Generated by breeze-generator (https://github.com/weibreeze/breeze-generator)
-- Schema: ` + schema.Name + `
-- Date: ` + time.Now().Format("2006/1/2\n"))
	buf.WriteString(`
local brz_w = require "resty.breeze.writer"
local brz_tools = require "resty.breeze.tools"
local brz_schema = require "resty.breeze.schema"
local brz_field_desc = require "resty.breeze.field_desc"
	`)

	fields := sortFields(message)

	msgName := schema.OrgPackage + "." + message.Name
	schemaField := ""

	// _M_t
	typeInits := ""
	writeFields := ""

	for _, field := range fields {
		schemaField += "    _m_schema:put_field(brz_field_desc(" + strconv.Itoa(field.Index) + ", '" + field.Name + "', '" + luaTypes[field.Type.Number].schemaTypeString + "'))\n"

		typeInits += "        " + field.Name + " = opts." + field.Name + luaTypes[field.Type.Number].defaultValue + ",\n"

		writeFields += getWriteFieldString(field)
	}
	if message.Alias != "" {
		schemaField += "_m_schema:set_alias(" + message.Alias + ")\n"
	}
	typeInits = typeInits[:len(typeInits)-2]
	writeFields = writeFields[:len(writeFields)-1]

	//class body
	buf.WriteString(`
local _M = {_VERSION = "` + LuaTemplateVersion + `"}
local _M_mt = {__index = _M}
	`)

	//construct
	buf.WriteString(`
function _M.new(self, opts)
    local _m_schema = brz_schema:new('` + msgName + `')

` + schemaField + `
    brz_tools:get_schema_seeker():add_schema('` + msgName + `', _m_schema)

    local _M_t = {
		_schema = _m_schema,
` + typeInits + `
    }
    return setmetatable(_M_t, _M_mt)
end`)

	//writeTo
	buf.WriteString(`
function _M.write_to(self, buf)
	return brz_w.write_msg_without_type(buf, function(fbuf)
` + writeFields + `
    end)
end
	`)

	//buf.Truncate(buf.Len() - 1)

	//end of class
	buf.WriteString(`
function _M.get_name(self)
    return self._schema:get_name()
end

function _M.is_breeze_msg(self)
    return true
end

return _M
	`)
	return withPackageDir(strings.ToLower(message.Name), schema, true) + ".lua", buf.Bytes(), nil
}

func (lt *LuaTemplate) generateEnum(schema *core.Schema, message *core.Message, context *core.Context) (file string, content []byte, err error) {
	buf := &bytes.Buffer{}

	buf.WriteString(`-- Generated by breeze-generator (https://github.com/weibreeze/breeze-generator)
-- Schema: ` + schema.Name + `
-- Date: ` + time.Now().Format("2006/1/2\n"))
	buf.WriteString(`
local brz_w = require "resty.breeze.writer"
local brz_tools = require "resty.breeze.tools"
local brz_schema = require "resty.breeze.schema"
local brz_field_desc = require "resty.breeze.field_desc"
	`)

	//class body
	buf.WriteString(`
local _M = {_VERSION = "` + LuaTemplateVersion + `"}
local _M_mt = {__index = _M}
	`)
	buf.WriteString("\n")

	// const
	enumValue := sortEnumValues(message)
	for _, value := range enumValue {
		buf.WriteString("local " + value.Name + " = " + strconv.Itoa(value.Index) + ";\n")
	}

	msgName := schema.OrgPackage + "." + message.Name
	schemaField := "    _m_schema:put_field(brz_field_desc(1, 'enumNumber', 'int32'))\n"
	if message.Alias != "" {
		schemaField += "_m_schema:set_alias(" + message.Alias + ")\n"
	}
	//construct
	buf.WriteString(`
function _M.new(self, enum_value)
	assert(type(enum_value) == "number", "enum number must be integer")
    local _m_schema = brz_schema:new('` + msgName + `')
` + schemaField + `
    brz_tools:get_schema_seeker():add_schema('` + msgName + `', _m_schema)

    local _M_t = {
		_schema = _m_schema,
		enumNumber = enum_value or 0
    }
    return setmetatable(_M_t, _M_mt)
end
	`)

	//end of class
	buf.WriteString(`

function _M.write_to(self, buf)
    return brz_w.write_msg_without_type(buf, function(fbuf)
        brz_w.write_int32_field(fbuf, 1, self.enumNumber)
    end)
end

function _M.value(self)
    return self.enumNumber
end

function _M.get_name(self)
    return self._schema:get_name()
end

function _M.is_breeze_msg(self)
    return true
end

return _M
	`)
	return withPackageDir(strings.ToLower(message.Name), schema, true) + ".lua", buf.Bytes(), nil
}

func (lt *LuaTemplate) generateService(schema *core.Schema, service *core.Service, context *core.Context) (file string, content []byte, err error) {
	//TODO implement
	return "", nil, nil
}

func (lt *LuaTemplate) generateMotanClient(schema *core.Schema, service *core.Service, context *core.Context) (file string, content []byte, err error) {
	//TODO implement
	return "", nil, nil
}

func (lt *LuaTemplate) getNamespace(pkg string) string {
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
