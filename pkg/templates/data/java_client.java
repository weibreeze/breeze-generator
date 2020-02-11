package {{.java_package}};

import com.weibo.api.motan.proxy.CommonHandler;
import com.weibo.api.motan.rpc.Request;

{{.imports}}

public class {{.service_name}}Client implements {{.service_name}} {
    private final CommonHandler commonHandler;
    private final String SERVICE_NAME = {{.service_name}}.class.getName();

    public {{.service_name}}Client(CommonHandler commonHandler) {
        this.commonHandler = commonHandler;
    }

   {{range $method := .methods}}
    @Override
    public {{$method.return_type_str}} {{$method.name}}({{$method.param_list_str}}) {
        try {
            Request request = commonHandler.buildRequest(
                    SERVICE_NAME,
                    "{{$method.name}}",
                    new Object[]{ {{$method.param_name_list_str}} }
            );
            return ({{$method.return_type_str}}) commonHandler.call(request, {{$method.return_type_str}}.class);
        } catch (Throwable throwable) {
            if (throwable instanceof RuntimeException) {
                throw (RuntimeException) throwable;
            } else {
                throw new RuntimeException(throwable);
            }
        }
    }
    {{end}}
}