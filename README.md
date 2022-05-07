# Breeze-Generator
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/weibreeze/breeze-generator/blob/master/LICENSE)
[![Build Status](https://img.shields.io/travis/weibreeze/breeze-generator/master.svg?label=Build)](https://travis-ci.org/weibreeze/breeze-generator)
[![codecov](https://codecov.io/gh/weibreeze/breeze-generator/branch/master/graph/badge.svg)](https://codecov.io/gh/weibreeze/breeze-generator)
[![GoDoc](https://godoc.org/github.com/weibreeze/breeze-generator?status.svg&style=flat)](https://godoc.org/github.com/weibreeze/breeze-generator)
[![Go Report Card](https://goreportcard.com/badge/github.com/weibreeze/breeze-generator)](https://goreportcard.com/report/github.com/weibreeze/breeze-generator)


# 概述
根据Breeze Schema生成各种语言的Breeze Message对象类。目前支持Java、PHP、Golang、C++。

# 快速入门

生成代码的样例如下：

```go
    func testGenerateCode() {
        path := "./main" // path can be a dir or a file
        config := &generator.Config{WritePath: "./autoGenerate", CodeTemplates: "php, go, java", Options: make(map[string]string)}
        result, err := generator.GeneratePath(path, config) // parse schema and generate code
        fmt.Printf("%v, %v\n", result, err)
    }
```

其中Config用来配置Schema解析和代码生成时的配置:

* `WritePath`用来指定生成代码的输出目录。

* `CodeTemplates`用来指定生成代码的语言，多种语言直接使用逗号分隔。如果需要对所有语言都生成，则可以使用`all`作为参数值。

* `Options`用来指定额外参数，例如针对不同语言生成模板的参数，比如`templates.GoPackagePrefix`用来指定go语言生成时统一的包前缀等。

具体代码可以参考[main/test.go](https://github.com/weibreeze/breeze-generator/blob/master/main/test.go)

## 做为Breeze生成服务器

可以使用`GenerateCodeHandler`做为http server来为Breeze的[intellij插件](https://github.com/weibreeze/breeze-idea-plugin) 提供生成服务。样例代码如下：


```go
    func main() {
        port := 8899
        path := "/generate_code"
        http.Handle(path, &breezeHttp.GenerateCodeHandler{})
        http.ListenAndServe(":"+strconv.Itoa(port), nil)
        select {}
    }
```




## 转换protobuf为breeze

生成器可以转换protobuf的.proto描述文件为breeze的.breeze描述文件。

但是有以下限制规则：

- 类型映射 double -> float64, float -> float32, [uint32, uint64] -> int64, sint32 -> int32, sint64 -> int64, [fixed32, fixed64] -> int64, sfixed32 -> int32, sfixed64 -> int64
- optional, required 忽略，字段默认值和拓展配置忽略。
- 不支持message，enum嵌套。
- 不支持import，extend，oneof，syntax，singular，repeated。
- 其他breeze没有的特性不支持。