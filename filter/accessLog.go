package filter

import (
	"time"

	motan "github.com/weibocom/motan-go/core"
	"github.com/weibocom/motan-go/log"
)

const (
	defaultRole     = "server"
	clientAgentRole = "client-agent"
	serverAgentRole = "server-agent"

	commonFormatLayout = "${request_id}|${service}|${method}|${desc}|${remote_addr}|${request_size}|${response_size}|${business_time}|${request_time}|${res_header.HTTP_Status}|${status}|${exception}"
)

var (
	defaultRoleAccessLogFormatter     = NewDefaultAccessLogFormatter(AccessLog + "|" + defaultRole + "|" + commonFormatLayout)
	clientAgentRoleAccessLogFormatter = NewDefaultAccessLogFormatter(AccessLog + "|" + clientAgentRole + "|" + commonFormatLayout)
	serverAgentRoleAccessLogFormatter = NewDefaultAccessLogFormatter(AccessLog + "|" + serverAgentRole + "|" + commonFormatLayout)
)

type AccessLogFilter struct {
	next motan.EndPointFilter
}

func (t *AccessLogFilter) GetIndex() int {
	return 1
}

func (t *AccessLogFilter) GetName() string {
	return AccessLog
}

func (t *AccessLogFilter) NewFilter(url *motan.URL) motan.Filter {
	return &AccessLogFilter{}
}

func (t *AccessLogFilter) Filter(caller motan.Caller, request motan.Request) motan.Response {
	roleAccessLogFormatter := defaultRoleAccessLogFormatter
	var ip string
	switch caller.(type) {
	case motan.Provider:
		roleAccessLogFormatter = serverAgentRoleAccessLogFormatter
		ip = request.GetAttachment(motan.HostKey)
	case motan.EndPoint:
		roleAccessLogFormatter = clientAgentRoleAccessLogFormatter
		ip = caller.GetURL().Host
	}
	remoteAddr := ip + ":" + caller.GetURL().GetPortStr()
	start := time.Now()
	response := t.GetNext().Filter(caller, request)
	request.GetRPCContext(true).RemoteAddress = remoteAddr
	request.GetRPCContext(true).RequestTime = time.Since(start).Nanoseconds() / 1e6
	vlog.RawAccessLog(roleAccessLogFormatter.Format(request, response))
	return response
}

func (t *AccessLogFilter) HasNext() bool {
	return t.next != nil
}

func (t *AccessLogFilter) SetNext(nextFilter motan.EndPointFilter) {
	t.next = nextFilter
}

func (t *AccessLogFilter) GetNext() motan.EndPointFilter {
	return t.next
}

func (t *AccessLogFilter) GetType() int32 {
	return motan.EndPointFilterType
}
