package http

import (
	"fmt"
	"net"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolver_LookupHost(t *testing.T) {
	resolver, err := NewResolver(filepath.Join("testdata", "resolv.conf"))
	if err != nil {
		t.Error(err)
	}
	ips, _ := resolver.LookupIP("127.0.0.1")
	assert.Equal(t, []net.IP{net.ParseIP("127.0.0.1")}, ips)
	fmt.Println(resolver.LookupHost("weibo.com"))
}
