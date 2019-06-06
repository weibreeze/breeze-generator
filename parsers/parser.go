package parsers

import (
	"strings"

	"github.com/weibreeze/breeze-generator/core"
)

//parser names
const (
	Breeze = "breeze"
)

//schema file suffix
const (
	BreezeFileSuffix = ".breeze"
)

var (
	UniformPackage = ""
)

var (
	instances = map[string]core.Parser{
		Breeze: &BreezeParser{},
	}
)

//GetParser get Parser by _name
func GetParser(name string) core.Parser {
	return instances[strings.ToLower(strings.TrimSpace(name))]
}

//Register : register a new parser
func Register(parser core.Parser) {
	instances[parser.Name()] = parser
}
