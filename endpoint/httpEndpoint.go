package endpoint

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
	"github.com/weibocom/motan-go/core"
	mhttp "github.com/weibocom/motan-go/http"
	"github.com/weibocom/motan-go/log"
)

const HTTPEndpointName = "httpEndpoint"

const (
	httpHealthCheckURI         = "checkURI"
	httpHealthCheckInterval    = "checkInterval"
	httpHealthCheckAliveStatus = "checkAliveStatus"
	httpHealthCheckRetryKey    = "checkRetry"
	httpHealthCheckTimeout     = "checkTimeout"
)

const (
	XForawrdedFor = "X-Forwarded-For"
)

const (
	httpDefaultRequestTimeout         = 1 * time.Second
	httpHealthCheckDefaultInterval    = 1 * time.Second
	httpHealthCheckDefaultTimeout     = 3 * time.Second
	httpHealthCheckDefaultAliveStatus = 200
	httpHealthCheckDefaultRetry       = 3
)

type HTTPEndpoint struct {
	url               *core.URL
	available         atomic.Value // bool
	httpClient        *fasthttp.HostClient
	defaultHTTPMethod string
	domain            string
	reverseProxy      bool

	healthCheckURL         string
	healthCheckInterval    time.Duration
	healthCheckTimeout     time.Duration
	healthCheckAliveStatus int
	healthCheckRetry       int

	destroyCh chan struct{}
}

func (h *HTTPEndpoint) Initialize() {
	timeout := h.url.GetTimeDuration(core.TimeOutKey, time.Millisecond, httpDefaultRequestTimeout)
	keepaliveTimeout := h.url.GetTimeDuration(mhttp.KeepaliveTimeoutKey, time.Millisecond, 5*time.Second)
	maxConnections := h.url.GetPositiveIntValue(core.MaxConnectionsKey, 50)
	h.domain = h.url.GetParam(mhttp.DomainKey, "")
	h.reverseProxy = h.url.GetString(core.NodeTypeKey) == core.NodeTypeService
	if getHTTPReqMethod, ok := h.url.Parameters["HTTP_REQUEST_METHOD"]; ok {
		h.defaultHTTPMethod = getHTTPReqMethod
	} else {
		h.defaultHTTPMethod = "GET"
	}
	h.httpClient = &fasthttp.HostClient{
		Name: "motan",
		Addr: h.url.Host + ":" + h.url.GetPortStr(),
		Dial: func(addr string) (net.Conn, error) {
			c, err := fasthttp.DialTimeout(addr, timeout)
			if err != nil {
				return c, err
			}
			return c, nil
		},
		MaxIdleConnDuration: keepaliveTimeout,
		MaxConns:            int(maxConnections),
		ReadTimeout:         timeout,
		WriteTimeout:        timeout,
	}
	h.destroyCh = make(chan struct{})
	checkURI := h.url.GetParam(httpHealthCheckURI, "")
	if checkURI != "" {
		if !strings.HasPrefix(checkURI, "/") {
			checkURI = "/" + checkURI
		}
		h.setAvailable(false)
		h.healthCheckURL = "http://" + h.url.Host + ":" + h.url.GetPortStr() + checkURI
		h.healthCheckInterval = h.url.GetTimeDuration(httpHealthCheckInterval, time.Millisecond, httpHealthCheckDefaultInterval)
		h.healthCheckTimeout = h.url.GetTimeDuration(httpHealthCheckTimeout, time.Millisecond, httpHealthCheckDefaultTimeout)
		h.healthCheckAliveStatus = int(h.url.GetPositiveIntValue(httpHealthCheckAliveStatus, httpHealthCheckDefaultAliveStatus))
		h.healthCheckRetry = int(h.url.GetPositiveIntValue(httpHealthCheckRetryKey, httpHealthCheckDefaultRetry))
	} else {
		h.setAvailable(true)
	}
	if h.healthCheckURL != "" {
		go h.checkStatus()
	}
}

func (h *HTTPEndpoint) checkStatus() {
	ticker := time.NewTicker(h.healthCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			alive := false
			for i := 0; i < h.healthCheckRetry; i++ {
				resp, err := http.Get(h.healthCheckURL)
				if err != nil {
					continue
				}
				if resp.StatusCode == h.healthCheckAliveStatus {
					alive = true
				}
				resp.Body.Close()
				break
			}
			if h.IsAvailable() {
				// available => unavailable
				if !alive {
					vlog.Infof("detect alive false, disable %s", h.url.GetIdentity())
				}
			} else {
				// unavailable => available
				if alive {
					vlog.Infof("detect alive true, enable %s", h.url.GetIdentity())
				}
			}
			h.setAvailable(alive)
		case <-h.destroyCh:
			return
		}
	}
}

func (h *HTTPEndpoint) GetName() string {
	return HTTPEndpointName
}

func (h *HTTPEndpoint) GetURL() *core.URL {
	return h.url
}

func (h *HTTPEndpoint) SetURL(url *core.URL) {
	h.url = url
}

func (h *HTTPEndpoint) setAvailable(status bool) {
	h.available.Store(status)
}

func (h *HTTPEndpoint) IsAvailable() bool {
	return h.available.Load().(bool)
}

func (h *HTTPEndpoint) Call(request core.Request) core.Response {
	startTime := time.Now().UnixNano()
	path := request.GetAttachment(mhttp.Path)
	if path == "" {
		path = request.GetMethod()
	}
	resp := &core.MotanResponse{
		RequestID:  request.GetRequestID(),
		Attachment: core.NewStringMap(core.DefaultAttachmentSize),
	}
	var toType []interface{}
	if err := request.ProcessDeserializable(toType); err != nil {
		core.BuildExceptionResponseWithCode(request, http.StatusBadRequest, startTime, err)
		return resp
	}
	doTransparentProxy, _ := strconv.ParseBool(request.GetAttachment(mhttp.Proxy))
	ip, xForwardedForIP := "", ""
	if h.reverseProxy {
		if remoteIP, exist := request.GetAttachments().Load(core.RemoteIPKey); exist {
			ip = remoteIP
		} else {
			ip = request.GetAttachment(core.HostKey)
		}
		request.GetAttachments().Range(func(k, v string) bool {
			// case insensitive compare
			if strings.EqualFold(k, XForawrdedFor) {
				xForwardedForIP = v
				return false
			}
			return true
		})
		if xForwardedForIP != "" {
			xForwardedForIP = xForwardedForIP + "," + ip
		} else {
			xForwardedForIP = ip
		}
	}

	httpReq := fasthttp.AcquireRequest()
	httpRes := fasthttp.AcquireResponse()
	httpReq.Header.DisableNormalizing()
	httpRes.Header.DisableNormalizing()
	defer fasthttp.ReleaseRequest(httpReq)
	defer fasthttp.ReleaseResponse(httpRes)

	if doTransparentProxy {
		var headerBytes, bodyBytes []byte
		reqHead := request.GetArguments()[0]
		if reqHead != nil {
			headerBytes = reqHead.([]byte)
		}
		reqBody := request.GetArguments()[1]
		if reqBody != nil {
			bodyBytes = reqBody.([]byte)
		}
		httpReq.Header.Read(bufio.NewReader(bytes.NewReader(headerBytes)))
		httpReq.URI().SetPath(path)
		httpReq.Header.Del("Connection")
		if h.reverseProxy {
			httpReq.Header.Set(XForawrdedFor, xForwardedForIP)
		}
		if len(bodyBytes) > 0 {
			httpReq.BodyWriter().Write(bodyBytes)
		}
		err := h.httpClient.Do(httpReq, httpRes)
		if err != nil {
			core.BuildExceptionResponseWithCode(request, http.StatusServiceUnavailable, startTime, err)
			return resp
		}
		headerBuffer := &bytes.Buffer{}
		httpRes.Header.Del("Connection")
		httpRes.Header.WriteTo(headerBuffer)
		resBody := httpRes.Body()
		// copy response body is needed
		responseBodyBytes := make([]byte, len(resBody))
		copy(responseBodyBytes, resBody)
		resp.Value = []interface{}{headerBuffer.Bytes(), responseBodyBytes}
		return resp
	}

	err := mhttp.MotanRequestToFasthttpRequest(request, httpReq, h.defaultHTTPMethod)
	if err != nil {
		core.BuildExceptionResponseWithCode(request, http.StatusBadRequest, startTime, err)
	}
	httpReq.URI().SetPath(path)
	if len(httpReq.Header.Host()) == 0 {
		httpReq.Header.SetHost(h.domain)
	}
	if h.reverseProxy {
		httpReq.Header.Set(XForawrdedFor, xForwardedForIP)
	}
	err = h.httpClient.Do(httpReq, httpRes)
	if err != nil {
		core.BuildExceptionResponseWithCode(request, http.StatusServiceUnavailable, startTime, err)
		return resp
	}
	mhttp.FasthttpResponseToMotanResponse(resp, httpRes)
	return resp
}

func (h *HTTPEndpoint) Destroy() {
	h.destroyCh <- struct{}{}
}

func (h *HTTPEndpoint) SetSerialization(s core.Serialization) {
}

func (h *HTTPEndpoint) SetProxy(proxy bool) {
}
