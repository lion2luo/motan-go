package filter

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weibocom/motan-go/core"
	"github.com/weibocom/motan-go/endpoint"
	"github.com/weibocom/motan-go/http"
)

func TestNewDefaultAccessLogFormatter(t *testing.T) {
	request := core.MotanRequest{}
	request.ServiceName = "test.service"
	request.Method = "test_method"
	request.RequestID = endpoint.GenerateRequestID()
	request.MethodDesc = "test_desc"
	request.GetRPCContext(true).RemoteAddress = "127.0.0.1:9981"
	request.GetRPCContext(true).BodySize = 1024
	request.GetRPCContext(true).RequestTime = 15
	response := core.MotanResponse{}
	response.RequestID = request.RequestID
	response.SetAttachment(http.Status, "200")
	response.SetProcessTime(10)
	response.GetRPCContext(true).BodySize = 1024

	var buffer bytes.Buffer
	buffer.WriteString("accessLog")
	buffer.WriteString("|")
	buffer.WriteString(defaultRole)
	buffer.WriteString("|")
	buffer.WriteString(strconv.FormatUint(request.RequestID, 10))
	buffer.WriteString("|")
	buffer.WriteString(request.GetServiceName())
	buffer.WriteString("|")
	buffer.WriteString(request.GetMethod())
	buffer.WriteString("|")
	buffer.WriteString(request.GetMethodDesc())
	buffer.WriteString("|")
	buffer.WriteString(request.GetRPCContext(true).RemoteAddress)
	buffer.WriteString("|")
	buffer.WriteString(strconv.Itoa(request.GetRPCContext(true).BodySize))
	buffer.WriteString("|")
	buffer.WriteString(strconv.Itoa(response.GetRPCContext(true).BodySize))
	buffer.WriteString("|")
	buffer.WriteString(strconv.FormatInt(response.GetProcessTime(), 10))
	buffer.WriteString("|")
	buffer.WriteString(strconv.FormatInt(request.GetRPCContext(true).RequestTime, 10))
	buffer.WriteString("|")
	buffer.WriteString("200")
	buffer.WriteString("|")
	buffer.WriteString(strconv.FormatBool(response.GetException() == nil))
	buffer.WriteString("|")

	exactedString := buffer.String()
	formattedString := defaultRoleAccessLogFormatter.Format(&request, &response)
	assert.Equal(t, exactedString, formattedString)
}
