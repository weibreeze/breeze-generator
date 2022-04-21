package motan

import (
	"bytes"
	"errors"
	"github.com/weibreeze/breeze-generator/core"
	"sort"
	"strings"
)

const (
	// motan config name
	defaultMotanBasicConfigName = "MotanBasicConfig"
	defaultRegistryName         = "MotanRegistry"
	defaultProtocolName         = "MotanProtocol"

	// motan config key
	basicConfigName           = "basicConfigName"
	serviceRegistryConfigName = "service.registryConfigName"
	serviceProtocolConfigName = "service.protocolConfigName"
	refererRegistryConfigName = "referer.registryConfigName"
	refererProtocolConfigName = "referer.protocolConfigName"

	motanConfigSuffix         = "MotanConfig"
	motanRegistryIdSuffix     = "Registry"
	motanProtocolIdSuffix     = "Protocol"
	motanBasicServiceIdSuffix = "BasicService"
	motanBasicRefererIdSuffix = "BasicReferer"

	defaultRegistryKeyPrefix = "default.registry." // default.开头的key，只允许出现在默认的basic config中
	defaultProtocolKeyPrefix = "default.protocol."
	refererConfigPrefix      = "referer."
	serviceConfigPrefix      = "service."
	serviceImplPrefix        = "serviceImpl."

	defaultMustSetValue = "${replacedMe}"
)

func BuildMotanConfig(schema *core.Schema) error {
	if schema.Options[core.WithMotanConfig] == "true" && len(schema.Services) > 0 {
		basicConfigMap := make(map[string]*core.Config)
		registryConfigMap := make(map[string]*core.Config)
		protocolConfigMap := make(map[string]*core.Config)
		serviceConfigMap := make(map[string]*core.Config)

		// default configs
		defaultBasicConfig := getConfigWithDefault(schema, defaultMotanBasicConfigName, nil)
		basicConfigMap[defaultMotanBasicConfigName] = defaultBasicConfig
		registryConfigMap[defaultRegistryName] = getConfigWithDefault(schema, defaultRegistryName, buildFromBasicConfig(defaultBasicConfig, defaultRegistryKeyPrefix))
		protocolConfigMap[defaultProtocolName] = getConfigWithDefault(schema, defaultProtocolName, buildFromBasicConfig(defaultBasicConfig, defaultProtocolKeyPrefix))

		// find all custom basic config, registry config, protocol config in service config
		for key := range schema.Services {
			config := getConfigWithDefault(schema, key+motanConfigSuffix, nil)
			serviceConfigMap[key] = config // service name as key
			err := findAndPutConfigByOptionKey(schema, config, basicConfigMap, basicConfigName)
			if err != nil {
				return err
			}
			err = findAndPutConfigByOptionKey(schema, config, registryConfigMap, serviceRegistryConfigName, refererRegistryConfigName)
			if err != nil {
				return err
			}
			err = findAndPutConfigByOptionKey(schema, config, protocolConfigMap, serviceProtocolConfigName, refererProtocolConfigName)
			if err != nil {
				return err
			}
		}

		// find all custom registry config, protocol config in basic config
		for _, config := range basicConfigMap {
			err := findAndPutConfigByOptionKey(schema, config, registryConfigMap, serviceRegistryConfigName, refererRegistryConfigName)
			if err != nil {
				return err
			}
			err = findAndPutConfigByOptionKey(schema, config, protocolConfigMap, serviceProtocolConfigName, refererProtocolConfigName)
			if err != nil {
				return err
			}
		}

		motanConfig := NewMotanConfig()
		completeRegistries(schema, motanConfig, registryConfigMap)
		completeProtocols(schema, motanConfig, protocolConfigMap)
		completeBasicConfigs(schema, motanConfig, basicConfigMap)
		completeServices(schema, motanConfig, serviceConfigMap)
		removeUselessDefaultConfig(motanConfig)
		schema.MotanConfig = motanConfig
	}
	return nil
}

func removeUselessDefaultConfig(config *core.MotanConfig) {
	if !config.NeedDefaultBasic {
		delete(config.BasicServices, defaultMotanBasicConfigName)
		delete(config.BasicReferers, defaultMotanBasicConfigName)
	}
	if !config.NeedDefaultRegistry {
		delete(config.Registries, defaultRegistryName)
	}
	if !config.NeedDefaultProtocol {
		delete(config.Protocols, defaultProtocolName)
	}
}

func getConfigWithDefault(schema *core.Schema, name string, defaultOptions map[string]string) *core.Config {
	config := schema.Configs[name]
	if config == nil {
		if defaultOptions == nil {
			defaultOptions = make(map[string]string)
		}
		config = &core.Config{Name: name, Options: defaultOptions}
	}
	return config
}

func findAndPutConfigByOptionKey(schema *core.Schema, config *core.Config, putTo map[string]*core.Config, keys ...string) error {
	for _, key := range keys {
		value := config.Options[key]
		if value != "" && putTo[value] == nil {
			c := schema.Configs[value]
			if c == nil {
				return errors.New("can not find config by option `" + key + "`. value:" + value)
			}
			putTo[value] = c
		}
	}
	return nil
}

// replace some kv，add missed registry kv
func completeRegistries(schema *core.Schema, motanConfig *core.MotanConfig, registryConfigMap map[string]*core.Config) {
	for name, config := range registryConfigMap {
		id := name                       // config name as default id if id not set
		if name == defaultRegistryName { // use common rule build default id for default registry
			id = buildDefaultId(motanRegistryIdSuffix, schema)
		}
		motanConfig.Registries[name] = fillDefaultRegistryKey(id, config.Options)
	}
}

// replace some kv，add missed registry kv
func completeProtocols(schema *core.Schema, motanConfig *core.MotanConfig, protocolConfigMap map[string]*core.Config) {
	for name, config := range protocolConfigMap {
		id := name
		if name == defaultProtocolName {
			id = buildDefaultId(motanProtocolIdSuffix, schema)
		}
		motanConfig.Protocols[name] = fillDefaultProtocolKey(id, config.Options)
	}
}

func completeBasicConfigs(schema *core.Schema, motanConfig *core.MotanConfig, basicConfigMap map[string]*core.Config) {
	for name, config := range basicConfigMap {
		serviceId := name + motanBasicServiceIdSuffix
		refererId := name + motanBasicRefererIdSuffix
		if name == defaultMotanBasicConfigName {
			serviceId = buildDefaultId(motanBasicServiceIdSuffix, schema)
			refererId = buildDefaultId(motanBasicRefererIdSuffix, schema)
		}
		basicConfig := fillDefaultBasicKey(serviceId, refererId, config.Options, motanConfig)
		basicServices, basicReferers, _ := buildServiceConfig(basicConfig)
		motanConfig.BasicServices[name] = basicServices
		motanConfig.BasicReferers[name] = basicReferers
	}
}

func completeServices(schema *core.Schema, motanConfig *core.MotanConfig, configMap map[string]*core.Config) {
	for name, config := range configMap {
		serviceConfig := fillDefaultServiceKey(schema, name, config.Options, motanConfig)
		services, referers, serviceImpls := buildServiceConfig(serviceConfig)
		motanConfig.Services[name] = services
		motanConfig.Referers[name] = referers
		motanConfig.ServiceImpls[name] = serviceImpls
	}
}

// build server end and client end basic config
func buildServiceConfig(configMap map[string]string) (services map[string]string, referers map[string]string, serviceImpls map[string]string) {
	services = make(map[string]string)
	referers = make(map[string]string)
	serviceImpls = make(map[string]string)
	for k, v := range configMap {
		if k == serviceProtocolConfigName || k == serviceRegistryConfigName ||
			k == refererProtocolConfigName || k == refererRegistryConfigName || k == basicConfigName {
			continue
		}
		if strings.HasPrefix(k, defaultRegistryKeyPrefix) || strings.HasPrefix(k, defaultProtocolKeyPrefix) {
			continue
		}
		if strings.HasPrefix(k, refererConfigPrefix) { // client end
			referers[removePrefix(k, refererConfigPrefix)] = v
		} else if strings.HasPrefix(k, serviceConfigPrefix) { //server end
			services[removePrefix(k, serviceConfigPrefix)] = v
		} else if strings.HasPrefix(k, serviceImplPrefix) { // service implement
			serviceImpls[removePrefix(k, serviceImplPrefix)] = v
		} else { // put in each config
			referers[k] = v
			services[k] = v
		}
	}
	return services, referers, serviceImpls
}

func NewMotanConfig() *core.MotanConfig {
	mc := &core.MotanConfig{}
	mc.Registries = make(map[string]map[string]string)
	mc.Protocols = make(map[string]map[string]string)
	mc.BasicServices = make(map[string]map[string]string)
	mc.BasicReferers = make(map[string]map[string]string)
	mc.Services = make(map[string]map[string]string)
	mc.Referers = make(map[string]map[string]string)
	mc.ServiceImpls = make(map[string]map[string]string)
	mc.NeedDefaultBasic = false
	mc.NeedDefaultProtocol = false
	mc.NeedDefaultRegistry = false
	return mc
}

func fillDefaultRegistryKey(defaultId string, registryMap map[string]string) map[string]string {
	putIfNotExist(registryMap, "id", defaultId)
	putIfNotExist(registryMap, "regProtocol", "vintage")
	putIfNotExist(registryMap, "address", defaultMustSetValue)
	putIfNotExist(registryMap, "port", "80")
	return registryMap
}

func fillDefaultProtocolKey(defaultId string, protocolMap map[string]string) map[string]string {
	putIfNotExist(protocolMap, "id", defaultId)
	putIfNotExist(protocolMap, "name", "motan2")
	putIfNotExist(protocolMap, "loadbalance", "configurableWeight")
	putIfNotExist(protocolMap, "haStrategy", "failover")
	putIfNotExist(protocolMap, "minWorkerThread", "200")
	putIfNotExist(protocolMap, "maxWorkerThread", "800")
	putIfNotExist(protocolMap, "minClientConnection", "2")
	putIfNotExist(protocolMap, "maxClientConnection", "10")
	putIfNotExist(protocolMap, "serialization", "breeze") // breeze as default
	return protocolMap
}

func fillDefaultBasicKey(serviceId string, refererId string, configMap map[string]string, motanConfig *core.MotanConfig) map[string]string {
	putIfNotExist(configMap, "referer.id", refererId)
	putIfNotExist(configMap, "service.id", serviceId)
	// set default registry if not set
	registryName := configMap[refererRegistryConfigName]
	if registryName == "" {
		registryName = defaultRegistryName
	}
	isPut := putIfNotExist(configMap, "referer.registry", motanConfig.Registries[registryName]["id"])
	if isPut && registryName == defaultRegistryName { // use default registry
		motanConfig.NeedDefaultRegistry = true
	}

	registryName = configMap[serviceRegistryConfigName]
	if registryName == "" {
		registryName = defaultRegistryName
	}
	isPut = putIfNotExist(configMap, "service.registry", motanConfig.Registries[registryName]["id"])
	if isPut && registryName == defaultRegistryName {
		motanConfig.NeedDefaultRegistry = true
	}

	// set default protocol if not set
	protocolName := configMap[refererProtocolConfigName]
	if protocolName == "" {
		protocolName = defaultProtocolName
	}
	isPut = putIfNotExist(configMap, "referer.protocol", motanConfig.Protocols[protocolName]["id"])
	if isPut && protocolName == defaultProtocolName {
		motanConfig.NeedDefaultProtocol = true
	}

	protocolName = configMap[serviceProtocolConfigName]
	if protocolName == "" {
		protocolName = defaultProtocolName
	}
	// random port by default
	isPut = putIfNotExist(configMap, "service.export", motanConfig.Protocols[protocolName]["id"]+":0")
	if isPut && protocolName == defaultProtocolName {
		motanConfig.NeedDefaultProtocol = true
	}
	v := configMap["service.export"]
	if strings.HasPrefix(v, ":") { // only has export port
		configMap["service.export"] = motanConfig.Protocols[protocolName]["id"] + v
		if protocolName == defaultProtocolName {
			motanConfig.NeedDefaultProtocol = true
		}
	}

	// other key with default
	putIfNotExist(configMap, "referer.requestTimeout", "1000")
	putIfNotExist(configMap, "referer.group", defaultMustSetValue)
	putIfNotExist(configMap, "referer.application", defaultMustSetValue)
	putIfNotExist(configMap, "referer.accessLog", "true")
	putIfNotExist(configMap, "service.group", defaultMustSetValue)
	putIfNotExist(configMap, "service.application", defaultMustSetValue)
	putIfNotExist(configMap, "service.accessLog", "true")
	return configMap
}

func fillDefaultServiceKey(schema *core.Schema, serviceName string, configMap map[string]string, motanConfig *core.MotanConfig) map[string]string {
	idPrefix := strings.ToLower(serviceName[:1]) + serviceName[1:] // first lower
	putIfNotExist(configMap, "referer.id", idPrefix)
	putIfNotExist(configMap, "service.id", idPrefix)

	// check custom basic config
	basicName := configMap[basicConfigName]
	if basicName == "" {
		basicName = defaultMotanBasicConfigName
	}
	isPut := putIfNotExist(configMap, "service.basicService", motanConfig.BasicServices[basicName]["id"])
	if isPut && basicName == defaultMotanBasicConfigName {
		motanConfig.NeedDefaultBasic = true
	}
	isPut = putIfNotExist(configMap, "referer.basicReferer", motanConfig.BasicReferers[basicName]["id"])
	if isPut && basicName == defaultMotanBasicConfigName {
		motanConfig.NeedDefaultBasic = true
	}

	// check custom registry
	registryName := configMap[refererRegistryConfigName]
	if registryName != "" {
		putIfNotExist(configMap, "referer.registry", motanConfig.Registries[registryName]["id"])
	}
	registryName = configMap[serviceRegistryConfigName]
	if registryName != "" {
		putIfNotExist(configMap, "service.registry", motanConfig.Registries[registryName]["id"])
	}

	// check custom protocol
	protocolName := configMap[refererProtocolConfigName]
	if protocolName != "" {
		putIfNotExist(configMap, "referer.protocol", motanConfig.Protocols[protocolName]["id"])
	}
	protocolName = configMap[serviceProtocolConfigName]
	if protocolName != "" {
		putIfNotExist(configMap, "service.export", motanConfig.Protocols[protocolName]["id"]+":0")
	}
	v := configMap["service.export"]
	if strings.HasPrefix(v, ":") { // only has export port
		if protocolName != "" {
			configMap["service.export"] = motanConfig.Protocols[protocolName]["id"] + v
		} else { // use basic config protocol if not set protocol config name
			basicExport := motanConfig.BasicServices[basicName]["export"]
			index := strings.Index(basicExport, ":")
			if index > 0 {
				configMap["service.export"] = basicExport[:index] + v
			}
		}
	}

	putIfNotExist(configMap, "interface", getInterface(schema, serviceName)) // interface for both service add referer
	putIfNotExist(configMap, "service.ref", idPrefix+"Impl")
	putIfNotExist(configMap, "serviceImpl.id", idPrefix+"Impl")
	putIfNotExist(configMap, "serviceImpl.class", getInterface(schema, serviceName)+"Impl")
	return configMap
}

func buildDefaultId(suffix string, schema *core.Schema) string {
	var prefix string
	if schema.Options[core.DefaultConfigIdPrefix] != "" {
		prefix = schema.Options[core.DefaultConfigIdPrefix]
	} else {
		prefix = removeSuffix(schema.Name, ".breeze")
	}
	return prefix + suffix
}

// return : true : put, false : not put
func putIfNotExist(m map[string]string, k string, v string) bool {
	if m[k] == "" {
		m[k] = v
		return true
	}
	return false
}

func buildFromBasicConfig(basicConfig *core.Config, prefix string) map[string]string {
	result := make(map[string]string)
	for k, v := range basicConfig.Options {
		if strings.HasPrefix(k, prefix) {
			result[removePrefix(k, prefix)] = v
		}
	}
	return result
}

func removePrefix(key string, prefix string) string {
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return key
}

func removeSuffix(key string, suffix string) string {
	if strings.HasSuffix(key, suffix) {
		return key[:len(key)-len(suffix)]
	}
	return key
}

func getInterface(schema *core.Schema, serviceName string) string {
	pkg := schema.Package
	ct := schema.Options[core.ConfigType]
	if ct == "" || ct == "xml" { // java
		if schema.Options[core.JavaPackage] != "" {
			pkg = schema.Options[core.JavaPackage]
		}
	}
	return pkg + "." + serviceName
}

func GenerateConfig(schema *core.Schema) (map[string][]byte, error) {
	contents := make(map[string][]byte)
	if schema.MotanConfig != nil && len(schema.MotanConfig.Services) > 0 {
		ct := schema.Options[core.ConfigType]
		if ct == "" || ct == "xml" {
			return generateXml(schema, contents)
		} else if ct == "yaml" {
			return generateYaml(schema, contents)
		}
	}
	return contents, nil
}

func generateXml(schema *core.Schema, contents map[string][]byte) (map[string][]byte, error) {
	buf := &bytes.Buffer{}
	header := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<beans xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n" +
		"       xmlns:motan=\"http://api.weibo.com/schema/motan\"\n" +
		"       xmlns=\"http://www.springframework.org/schema/beans\"\n" +
		"       xsi:schemaLocation=\"http://www.springframework.org/schema/beans http://www.springframework.org/schema/beans/spring-beans-2.5.xsd\n" +
		"       http://api.weibo.com/schema/motan http://api.weibo.com/schema/motan.xsd\">\n"

	// ------- build server end ---------
	// add xml header
	buf.WriteString(header)
	//add service impl
	for _, conf := range schema.MotanConfig.ServiceImpls {
		buf.WriteString("\n    <!-- service implement beans -->\n")
		buf.WriteString("    <bean id=\"") // id at first
		buf.WriteString(conf["id"])
		buf.WriteString("\"")
		putXmlAttribute(buf, conf, "          ", "id")
	}
	// add registry and protocol
	putRegistryAndProtocolWithXml(buf, schema.MotanConfig)
	// add basic service
	for _, conf := range schema.MotanConfig.BasicServices {
		buf.WriteString("\n    <!-- basic service configs -->\n")
		buf.WriteString("    <motan:basicService id=\"")
		buf.WriteString(conf["id"])
		buf.WriteString("\"")
		putXmlAttribute(buf, conf, "                        ", "id")
	}
	// add service
	for _, conf := range schema.MotanConfig.Services {
		buf.WriteString("\n    <!-- service configs -->\n")
		buf.WriteString("    <motan:service")
		putXmlAttribute(buf, conf, "                   ", "id")
	}
	buf.WriteString("</beans>\n")
	contents[removeSuffix(schema.Name, ".breeze")+"-rpc.xml"] = buf.Bytes()

	// ------- build client end ---------
	buf = &bytes.Buffer{}
	buf.WriteString(header)
	putRegistryAndProtocolWithXml(buf, schema.MotanConfig)
	//add basic referer
	for _, conf := range schema.MotanConfig.BasicReferers {
		buf.WriteString("\n    <!-- basic referer configs -->\n")
		buf.WriteString("    <motan:basicReferer id=\"")
		buf.WriteString(conf["id"])
		buf.WriteString("\"")
		putXmlAttribute(buf, conf, "                        ", "id")
	}
	// add referer
	for _, conf := range schema.MotanConfig.Referers {
		buf.WriteString("\n    <!-- referer configs -->\n")
		buf.WriteString("    <motan:referer id=\"")
		buf.WriteString(conf["id"])
		buf.WriteString("\"")
		putXmlAttribute(buf, conf, "                   ", "id")
	}
	buf.WriteString("</beans>\n")
	contents[removeSuffix(schema.Name, ".breeze")+"-rpc-client.xml"] = buf.Bytes()
	return contents, nil
}

func putRegistryAndProtocolWithXml(buf *bytes.Buffer, motanConfig *core.MotanConfig) {
	// add registry
	for _, conf := range motanConfig.Registries {
		buf.WriteString("\n    <!-- registry configs -->\n")
		buf.WriteString("    <motan:registry id=\"")
		buf.WriteString(conf["id"])
		buf.WriteString("\"")
		putXmlAttribute(buf, conf, "                    ", "id")
	}
	// add protocol
	for _, conf := range motanConfig.Protocols {
		buf.WriteString("\n    <!-- protocol configs -->\n")
		buf.WriteString("    <motan:protocol id=\"")
		buf.WriteString(conf["id"])
		buf.WriteString("\"")
		putXmlAttribute(buf, conf, "                    ", "id")
	}
}

// NOTICE: exclude key size is used as start index for new line. if exclude keys are not equals with keys already wrote in buf, add new param for start index
func putXmlAttribute(buf *bytes.Buffer, conf map[string]string, indent string, excludes ...string) {
	i := len(excludes) // used for start new line
	for _, k := range sortKeys(conf) {
		ignore := false
		for _, ek := range excludes {
			if k == ek {
				ignore = true
				break
			}
		}
		if !ignore {
			if i != 0 && i%3 == 0 {
				buf.WriteString("\n" + indent)
			} else {
				buf.WriteString(" ")
			}
			buf.WriteString(k)
			buf.WriteString("=\"")
			buf.WriteString(conf[k])
			buf.WriteString("\"")
			i++
		}
	}
	buf.WriteString("/>\n")
}

func putYamlAttribute(buf *bytes.Buffer, conf map[string]string, excludes ...string) {
	for _, k := range sortKeys(conf) {
		ignore := false
		for _, ek := range excludes {
			if k == ek {
				ignore = true
				break
			}
		}
		if !ignore {
			buf.WriteString("    " + k + ": " + conf[k] + "\n")
		}
	}
}

func sortKeys(m map[string]string) []string {
	keys := make([]string, 0, 16)
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func generateYaml(schema *core.Schema, contents map[string][]byte) (map[string][]byte, error) {
	buf := &bytes.Buffer{}
	// ------- build server end ---------
	// registry
	buf.WriteString("motan-registry:\n")
	for _, conf := range schema.MotanConfig.Registries {
		buf.WriteString("  " + conf["id"] + ":\n")
		putYamlAttribute(buf, conf, "id")
	}
	// basic service
	putService(buf, "\nmotan-basicService:\n", schema.MotanConfig.BasicServices, schema.MotanConfig)
	// service
	putService(buf, "\nmotan-service:\n", schema.MotanConfig.Services, schema.MotanConfig)

	contents[removeSuffix(schema.Name, ".breeze")+"-rpc.yaml"] = buf.Bytes()

	// ------- build client end ---------
	buf = &bytes.Buffer{}
	// registry
	buf.WriteString("motan-registry:\n")
	for _, conf := range schema.MotanConfig.Registries {
		buf.WriteString("  " + conf["id"] + ":\n")
		putYamlAttribute(buf, conf, "id")
	}
	// basic referer
	putReferer(buf, "\nmotan-basicRefer:\n", schema.MotanConfig.BasicReferers, schema.MotanConfig)
	putReferer(buf, "\nmotan-refer:\n", schema.MotanConfig.Referers, schema.MotanConfig)
	contents[removeSuffix(schema.Name, ".breeze")+"-rpc-client.yaml"] = buf.Bytes()
	return contents, nil
}

func putReferer(buf *bytes.Buffer, sectionKey string, refererConfig map[string]map[string]string, motanConfig *core.MotanConfig) {
	buf.WriteString(sectionKey)
	for _, conf := range refererConfig {
		buf.WriteString("  " + conf["id"] + ":\n")
		// merge protocol config
		c := make(map[string]string)
		for k, v := range conf {
			c[k] = v
		}
		if conf["protocol"] != "" {
			mergeProtocol(c, conf["protocol"], motanConfig)
		}
		if c["interface"] != "" { // replace 'interface' by 'path'
			c["path"] = c["interface"]
		}
		putYamlAttribute(buf, c, "id", "interface")
	}
}

func putService(buf *bytes.Buffer, sectionKey string, serviceConfig map[string]map[string]string, motanConfig *core.MotanConfig) {
	buf.WriteString(sectionKey)
	for _, conf := range serviceConfig {
		buf.WriteString("  " + conf["id"] + ":\n")
		// merge protocol config
		c := make(map[string]string)
		for k, v := range conf {
			c[k] = v
		}
		if index := strings.Index(conf["export"], ":"); index > 0 {
			mergeProtocol(c, conf["export"][:index], motanConfig)
		}
		if c["interface"] != "" { // replace 'interface' by 'path'
			c["path"] = c["interface"]
		}
		putYamlAttribute(buf, c, "id", "interface")
	}
}

func mergeProtocol(conf map[string]string, protocolId string, motanConfig *core.MotanConfig) {
	for _, pc := range motanConfig.Protocols {
		if protocolId == pc["id"] { // find protocol config
			for pk, pv := range pc {
				if pk != "id" {
					putIfNotExist(conf, pk, pv)
				}
			}
		}
	}
}
