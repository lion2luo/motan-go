package filter

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"github.com/weibocom/motan-go/core"
)

type AccessLogFormatter interface {
	Format(req core.Request, res core.Response) string
}

type layoutValueFinder interface {
	find(req core.Request, res core.Response) string
}

type layoutValueFinderFunc func(req core.Request, res core.Response) string

func (f layoutValueFinderFunc) find(req core.Request, res core.Response) string {
	return f(req, res)
}

var (
	formatBufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
	acquireBuffer = func() *bytes.Buffer {
		buffer := formatBufferPool.Get().(*bytes.Buffer)
		buffer.Reset()
		return buffer
	}
	releaseBuffer = func(buffer *bytes.Buffer) {
		formatBufferPool.Put(buffer)
	}

	requestIDFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return strconv.FormatUint(res.GetRequestID(), 10)
	})
	requestTimeFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return strconv.FormatInt(req.GetRPCContext(true).RequestTime, 10)
	})
	serviceFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return req.GetServiceName()
	})
	methodFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return req.GetMethod()
	})
	descFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return req.GetMethodDesc()
	})
	remoteAddressFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return req.GetRPCContext(true).RemoteAddress
	})
	requestSizeFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return strconv.Itoa(req.GetRPCContext(true).BodySize)
	})
	responseSizeFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return strconv.Itoa(res.GetRPCContext(true).BodySize)
	})
	businessTimeFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return strconv.FormatInt(res.GetProcessTime(), 10)
	})
	statusFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		return strconv.FormatBool(res.GetException() == nil)
	})
	exceptionFinder layoutValueFinder = layoutValueFinderFunc(func(req core.Request, res core.Response) string {
		ex := res.GetException()
		if ex != nil {
			bytes, _ := json.Marshal(ex)
			return string(bytes)
		}
		return ""
	})
)

type requestAttachmentFinder struct {
	attachmentName string
}

func (f *requestAttachmentFinder) find(req core.Request, res core.Response) string {
	return req.GetAttachment(f.attachmentName)
}

type responseAttachmentFinder struct {
	attachmentName string
}

func (f *responseAttachmentFinder) find(req core.Request, res core.Response) string {
	return res.GetAttachment(f.attachmentName)
}

type stringFinder struct {
	s string
}

func (s *stringFinder) find(req core.Request, res core.Response) string {
	return s.s
}

const (
	requestAttachmentPrefix  = "req_header."
	responseAttachmentPrefix = "res_header."
)

func getVariableFinder(varName string) layoutValueFinder {
	switch varName {
	case "request_id":
		return requestIDFinder
	case "request_time":
		return requestTimeFinder
	case "service":
		return serviceFinder
	case "method":
		return methodFinder
	case "desc":
		return descFinder
	case "remote_addr":
		return remoteAddressFinder
	case "request_size":
		return requestSizeFinder
	case "response_size":
		return responseSizeFinder
	case "business_time":
		return businessTimeFinder
	case "status":
		return statusFinder
	case "exception":
		return exceptionFinder
	default:
		if strings.HasPrefix(varName, requestAttachmentPrefix) {
			return &requestAttachmentFinder{attachmentName: varName[len(requestAttachmentPrefix):]}
		}
		if strings.HasPrefix(varName, responseAttachmentPrefix) {
			return &responseAttachmentFinder{attachmentName: varName[len(responseAttachmentPrefix):]}
		}
		return &stringFinder{s: "${" + varName + "}"}
	}
}

func getValueFindersFromLayout(layout string) []layoutValueFinder {
	paringVariable := false
	val := bytes.Buffer{}
	layoutRunes := []rune(layout)
	layoutLen := len(layoutRunes)
	var valueFinders []layoutValueFinder
	for i := 0; i < layoutLen; i++ {
		c := layoutRunes[i]
		switch c {
		case '$':
			if paringVariable {
				val.WriteRune(c)
				break
			}
			if i < layoutLen-1 && layoutRunes[i+1] == '{' {
				paringVariable = true
				i++
				// we need flush val here for a variable begin
				vs := val.String()
				if vs != "" {
					valueFinders = append(valueFinders, &stringFinder{s: vs})
				}
				val.Reset()
				break
			}
			val.WriteRune(c)
		case '}':
			// end of variable
			if !paringVariable {
				val.WriteRune(c)
				break
			}
			varName := val.String()
			// if no variable name ignore it
			if varName != "" {
				valueFinders = append(valueFinders, getVariableFinder(varName))
			}
			val.Reset()
			paringVariable = false
		default:
			val.WriteRune(c)
		}
	}
	if paringVariable {
		// if still in paring variable after all characters handled
		// we treat the val buffer as a string, so we need add ${ here
		s := "${" + val.String()
		valueFinders = append(valueFinders, &stringFinder{s: s})
	} else {
		vs := val.String()
		if vs != "" {
			valueFinders = append(valueFinders, &stringFinder{s: vs})
		}
	}
	return valueFinders
}

type DefaultAccessLogFormatter struct {
	layout       string
	valueFinders []layoutValueFinder
}

func (f *DefaultAccessLogFormatter) Format(req core.Request, res core.Response) string {
	buffer := acquireBuffer()
	defer releaseBuffer(buffer)
	for _, f := range f.valueFinders {
		buffer.WriteString(f.find(req, res))
	}
	return buffer.String()
}

// format like following
// ${request_id}|${service}|${method}|${desc}|${business_time}|${request_size}|${response_size}|${req_header.attachment_name)|${res_header.attachment_name}}
func NewDefaultAccessLogFormatter(layout string) AccessLogFormatter {
	return &DefaultAccessLogFormatter{
		layout:       layout,
		valueFinders: getValueFindersFromLayout(layout),
	}
}
