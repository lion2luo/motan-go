package endpoint

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
	"github.com/weibocom/motan-go/core"
	mhttp "github.com/weibocom/motan-go/http"
)

var lock sync.Mutex
var statusCode int

func setStatusCode(code int) {
	lock.Lock()
	defer lock.Unlock()
	statusCode = code
}

func getStatusCode() int {
	lock.Lock()
	defer lock.Unlock()
	return statusCode
}

func TestMain(m *testing.M) {
	go func() {
		var addr = ":9090"
		handler := &http.ServeMux{}
		handler.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			request.ParseForm()
			writer.WriteHeader(getStatusCode())
			writer.Write([]byte(request.URL.String()))

		})
		http.ListenAndServe(addr, handler)
	}()
	os.Exit(m.Run())
}

func TestHTTPEndpoint_IsAvailable(t *testing.T) {
	url := &core.URL{}
	url.Protocol = "http"
	url.Host = "localhost"
	url.Port = 9090
	url.PutParam(mhttp.DomainKey, "test.domain")
	url.PutParam(httpHealthCheckURI, "/")
	url.PutParam(httpHealthCheckInterval, "20")
	endpoint := HTTPEndpoint{url: url}
	endpoint.Initialize()
	setStatusCode(200)
	time.Sleep(200 * time.Millisecond)
	fmt.Println(endpoint.IsAvailable())
	setStatusCode(503)
	time.Sleep(200 * time.Millisecond)
	fmt.Println(endpoint.IsAvailable())
	setStatusCode(200)
	time.Sleep(200 * time.Millisecond)

	req := &core.MotanRequest{}
	req.ServiceName = "test"
	req.Method = "test"
	req.SetAttachment(mhttp.QueryString, "a=b")
	assert.Equal(t, "/test?a=b", string(endpoint.Call(req).GetValue().([]byte)))

	req.SetAttachment(mhttp.Proxy, "true")
	httpReq := fasthttp.AcquireRequest()
	httpReq.Header.SetMethod("GET")
	httpReq.SetRequestURI("/test?a=b")
	httpReq.Header.Set("Host", "test.domain")
	headerBuffer := &bytes.Buffer{}
	httpReq.Header.WriteTo(headerBuffer)
	req.Arguments = []interface{}{headerBuffer.Bytes(), nil}
	assert.Equal(t, "/test?a=b", string(endpoint.Call(req).GetValue().([]interface{})[1].([]byte)))
}
