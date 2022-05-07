package http

import (
	"encoding/json"
	"fmt"
	generator "github.com/weibreeze/breeze-generator"
	"github.com/weibreeze/breeze-generator/core"
	"net/http"
)

type GenerateRes struct {
	Result        bool              `json:"result"`
	ErrMsg        string            `json:"err_msg"`
	CodeContent   map[string]string `json:"code_content"`
	ConfigContent map[string]string `json:"config_content"`
}

// GenerateCodeHandler 处理生成代码逻辑
type GenerateCodeHandler struct {
}

func (s *GenerateCodeHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	res := &GenerateRes{}
	targetLanguage := req.FormValue("target_language")
	optionsStr := req.FormValue("options")
	fileContentStr := req.FormValue("file_content")
	if fileContentStr != "" {
		fileMap := make(map[string]string)
		optionsMap := make(map[string]string)
		json.Unmarshal([]byte(fileContentStr), &fileMap)
		if optionsStr != "" {
			json.Unmarshal([]byte(optionsStr), &optionsMap)
		}
		if optionsMap[core.WithPackageDir] == "" {
			optionsMap[core.WithPackageDir] = "true"
		}
		config := &generator.Config{WriteFile: false, CodeTemplates: targetLanguage, Options: optionsMap}
		var err error
		res.CodeContent, res.ConfigContent, err = generator.GeneratByFileContent(fileMap, config)
		if err == nil {
			res.Result = true
		} else {
			res.ErrMsg = err.Error()
		}
	} else {
		res.ErrMsg = "param file_content should not empty"
	}

	// build return json
	bytes, err := json.Marshal(res)
	if err == nil {
		rw.Write(bytes)
		return
	} else {
		fmt.Errorf("GenerateCodeHandler: encode json fail. err: {%s}", err.Error())
		rw.Write([]byte("{\"result\":false,\"err_msg\":\"encode json fail." + err.Error() + "\"}"))
	}
}
