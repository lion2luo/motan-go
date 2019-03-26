package provider

import (
	"errors"
	"net/http"
	"time"

	"github.com/weibocom/motan-go/cluster"
	"github.com/weibocom/motan-go/core"
	mhttp "github.com/weibocom/motan-go/http"
	"github.com/weibocom/motan-go/log"
)

const (
	UpstreamRegistryKey = "upstreamRegistry"
	UpstreamProtocolKey = "upstreamProtocol"
)

// ReverseProxyProvider struct
type ReverseProxyProvider struct {
	url              *core.URL
	configContext    *core.Context
	extensionFactory core.ExtensionFactory
	motanCluster     *cluster.MotanCluster

	domain          string
	locationMatcher *mhttp.LocationMatcher
	upstreamIsHTTP  bool
}

// Initialize ReverseProxyProvider
func (h *ReverseProxyProvider) Initialize() {
	clusterURL := h.url.Copy()
	clusterURL.Host = ""
	clusterURL.Port = 0
	clusterURL.Protocol = clusterURL.GetParam(UpstreamProtocolKey, "motan2")
	// TODO: we need to do url rewrite for http, but here maybe not good place to handle this
	if clusterURL.Protocol == "http" {
		h.upstreamIsHTTP = true
		h.domain = h.url.GetParam(mhttp.DomainKey, "")
		h.locationMatcher = mhttp.NewLocationMatcherFromContext(h.domain, h.configContext)
	}
	upstreamRegistry := clusterURL.GetParam(UpstreamRegistryKey, "")
	if upstreamRegistry == "" {
		vlog.Errorf("When use a http mesh provider you should configure [" + UpstreamRegistryKey + "] to specify how to get nodes")
		return
	}
	clusterURL.RemoveParam(UpstreamRegistryKey)
	clusterURL.PutParam(core.RegistryKey, upstreamRegistry)
	h.motanCluster = cluster.NewCluster(h.configContext, h.extensionFactory, clusterURL, true)
}

// Destroy a ReverseProxyProvider
func (h *ReverseProxyProvider) Destroy() {
	h.motanCluster.Destroy()
}

// SetSerialization for set a motan.SetSerialization to ReverseProxyProvider
func (h *ReverseProxyProvider) SetSerialization(s core.Serialization) {
}

// SetProxy for ReverseProxyProvider
func (h *ReverseProxyProvider) SetProxy(proxy bool) {
}

// SetContext use to set global config to ReverseProxyProvider
func (h *ReverseProxyProvider) SetContext(context *core.Context) {
	h.configContext = context
}

// Call for do a motan call through this provider
func (h *ReverseProxyProvider) Call(request core.Request) core.Response {
	startTime := time.Now().UnixNano()
	if h.upstreamIsHTTP {
		_, rewritePath, ok := h.locationMatcher.Pick(request.GetMethod(), true)
		if !ok {
			return core.BuildExceptionResponseWithCode(request, http.StatusServiceUnavailable, startTime, errors.New("service not found"))
		}
		request.SetAttachment(mhttp.Path, rewritePath)
	}
	return h.motanCluster.Call(request)
}

// GetName return this provider name
func (h *ReverseProxyProvider) GetName() string {
	return "ReverseProxyProvider"
}

// GetURL return the url that represent for this provider
func (h *ReverseProxyProvider) GetURL() *core.URL {
	return h.url
}

// SetURL to set a motan to represent for this provider
func (h *ReverseProxyProvider) SetURL(url *core.URL) {
	h.url = url
}

// IsAvailable to check if this provider is still working well
func (h *ReverseProxyProvider) IsAvailable() bool {
	return true
}

// SetService to set services to this provider that which can handle
func (h *ReverseProxyProvider) SetService(s interface{}) {
}

// GetPath return current url path from the provider's url
func (h *ReverseProxyProvider) GetPath() string {
	return h.url.Path
}
