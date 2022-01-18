package generator

import (
	assert2 "github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

var (
	txt = `
// You use definitions from old.proto and new.proto, but not other.proto
option java_package = "com.example.foo";
//go_package_prefix
option go_package = "com.example.foo";
option cc_enable_arenas = true;
import "myproject/other_protos.proto";
message SearchRequest {
	extend Foo {
    optional int32 bar = 126;
  }
  repeated int32 bar = 126;
  required string query = 1;
   // You use definitions from old.proto and new.proto, but not other.proto
  optional int32 page_number = 2 [packed=true];
  optional int32 result_per_page = 3 [packed=true];
  float d1=4;

message MiddleAA {  // Level 1
     float d1=4;
  }
  message MiddleBB {  // Level 1
    
}
  message MiddleCC {  // Level 1
    
}
	extensions 100 to 199;
	oneof test_oneof {
     string name = 4;
     SubMessage sub_message = 9;
  }
}

oneof test_oneof {
     string name = 4;
     SubMessage sub_message = 9;
}
message OutDD {  // Level 1
    
}
service CollaborativeMutationService {
  rpc GetUidMutation(CollaborativeMutationRequest) returns (CollaborativeMutationResponse);
  rpc GetUidMutationCategory(CollaborativeMutationRequest) returns (CollaborativeMutationResponse);
}
`
)

func TestProtoToBreeze(t *testing.T) {
	assert := assert2.New(t)

	txt = protoToBreeze(txt)
	for _, v := range []string{
		"double", "float ", "uint32", "uint64", "sint32", "sint64", "fixed32", "fixed64", "sfixed32", "sfixed64",
		"oneof", "extensions", "MiddleCC", "MiddleBB", "MiddleAA", "optional", "required", "//", "extend", "import",
	} {
		assert.NotContains(txt, v)
	}
	for _, v := range []string{
		"message OutDD {", "message SearchRequest {", "}", "go_package_prefix","CollaborativeMutationRequest request",
	} {
		assert.Contains(txt, v)
	}
	//fmt.Println(txt)
}
func TestProtoToBreeze_1(t *testing.T) {
	assert := assert2.New(t)
	root := ".test_ProtoToBreeze_1"
	src := root + "/src"
	dest := root + "/dest"
	os.RemoveAll(root)
	os.MkdirAll(src, 0755)
	os.MkdirAll(dest, 0755)
	ioutil.WriteFile(src+"/any.proto",[]byte(txt),0755)
	assert.DirExists(src)
	assert.DirExists(dest)
	assert.FileExists(src+"/any.proto")
	ProtoToBreeze(src,dest)
	assert.FileExists(dest+"/any.breeze")
	os.RemoveAll(root)
}

//func TestProtoToBreeze_2(t *testing.T) {
//	root := ".test_ProtoToBreeze_1"
//	src :=  "/Users/user/Desktop/test/proto"
//	dest := root + "/dest"
//	os.RemoveAll(root)
//	os.MkdirAll(dest, 0755)
//	ProtoToBreeze(src,dest)
//}
//
//func TestProtoToBreeze_3(t *testing.T) {
//	os.RemoveAll("auto")
//	config := &Config{WritePath: "auto", CodeTemplates: "all", Options: make(map[string]string)}
//	config.Options[core.WithPackageDir] = "true"
//	_,err:=GeneratePath(".test_ProtoToBreeze_1/dest", config)
//	fmt.Println(err)
//}

