package filter

import (
	"time"

	motan "github.com/weibocom/motan-go/core"
	vlog "github.com/weibocom/motan-go/log"
)

var (
	clusterAccessLogFormatter = NewDefaultAccessLogFormatter(ClusterAccessLog + "|" + clientAgentRole + "|" + commonFormatLayout)
)

type ClusterAccessLogFilter struct {
	next motan.ClusterFilter
}

func (t *ClusterAccessLogFilter) GetIndex() int {
	return 1
}

func (t *ClusterAccessLogFilter) GetName() string {
	return ClusterAccessLog
}

func (t *ClusterAccessLogFilter) NewFilter(url *motan.URL) motan.Filter {
	return &ClusterAccessLogFilter{}
}

func (t *ClusterAccessLogFilter) Filter(haStrategy motan.HaStrategy, loadBalance motan.LoadBalance, request motan.Request) motan.Response {
	start := time.Now()
	response := t.GetNext().Filter(haStrategy, loadBalance, request)
	request.GetRPCContext(true).RequestTime = time.Since(start).Nanoseconds() / 1e6
	vlog.RawAccessLog(clientAgentRoleAccessLogFormatter.Format(request, response))
	return response
}

func (t *ClusterAccessLogFilter) HasNext() bool {
	return t.next != nil
}

func (t *ClusterAccessLogFilter) SetNext(nextFilter motan.ClusterFilter) {
	t.next = nextFilter
}

func (t *ClusterAccessLogFilter) GetNext() motan.ClusterFilter {
	return t.next
}

func (t *ClusterAccessLogFilter) GetType() int32 {
	return motan.ClusterFilterType
}
