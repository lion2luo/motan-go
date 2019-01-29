package http

import (
	"errors"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type resolveConfig struct {
	modifyTime  time.Time
	dnsConfig   *dns.ClientConfig
	lastChecked time.Time
	resolveConf string
	ch          chan struct{}
	mu          sync.RWMutex
}

func newResolveConfig(resolveConf string) (*resolveConfig, error) {
	confInfo, err := os.Stat(resolveConf)
	if err != nil {
		return nil, err
	}
	dnsConfig, err := dns.ClientConfigFromFile(resolveConf)
	if err != nil {
		return nil, err
	}
	conf := resolveConfig{dnsConfig: dnsConfig,
		modifyTime:  confInfo.ModTime(),
		lastChecked: time.Now(),
		resolveConf: resolveConf}
	conf.ch = make(chan struct{}, 1)
	return &conf, nil
}

func (conf *resolveConfig) tryAcquireSema() bool {
	select {
	case conf.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

func (conf *resolveConfig) releaseSema() {
	<-conf.ch
}

// tryUpdateConfig update resolve configuration if need
// See net/dnsclient_unix.go
func (conf *resolveConfig) tryUpdateConfig() {
	if !conf.tryAcquireSema() {
		return
	}
	defer conf.releaseSema()

	now := time.Now()
	if conf.lastChecked.After(now.Add(-5 * time.Second)) {
		return
	}
	conf.lastChecked = now
	var mtime time.Time
	if fi, err := os.Stat(conf.resolveConf); err == nil {
		mtime = fi.ModTime()
	}
	if mtime.Equal(conf.modifyTime) {
		return
	}

	if config, err := dns.ClientConfigFromFile(conf.resolveConf); err == nil {
		conf.mu.Lock()
		conf.dnsConfig = config
		conf.mu.Unlock()
	}
}

type Resolver struct {
	config *resolveConfig
	client *dns.Client
}

func NewResolver(resolveConf string) (*Resolver, error) {
	if resolveConf == "" {
		resolveConf = "/etc/resolv.conf"
	}
	resolver := Resolver{}
	config, err := newResolveConfig(resolveConf)
	if err != nil {
		return nil, err
	}
	resolver.config = config
	resolver.client = &dns.Client{}
	return &resolver, nil
}

func (r *Resolver) LookupIP(host string) ([]net.IP, error) {
	addrs, err := r.LookupHost(host)
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, addr := range addrs {
		ips = append(ips, net.ParseIP(addr))
	}
	return ips, nil
}

func (r *Resolver) LookupHost(host string) ([]string, error) {
	if host == "" {
		return nil, errors.New("no such host")
	}
	if ip := net.ParseIP(host); ip != nil {
		return []string{host}, nil
	}
	return r.lookupHost(host)
}

func (r *Resolver) lookupHost(host string) ([]string, error) {
	if _, ok := dns.IsDomainName(host); !ok {
		return nil, errors.New("no such host " + host)
	}
	r.config.tryUpdateConfig()
	msg := new(dns.Msg)
	// TODO: add ipv6 support with type dns.TypeAAAA
	msg.SetQuestion(host+".", dns.TypeA)
	r.config.mu.RLock()
	config := r.config
	r.config.mu.RUnlock()
	servers := config.dnsConfig.Servers
	server := servers[rand.Intn(len(servers))]
	if strings.IndexByte(server, ':') == -1 || (server[0] == '[' && server[len(server)-1] == ']') {
		server = server + ":53"
	}
	in, _, err := r.client.Exchange(msg, server)
	if err != nil {
		return nil, err
	}
	var addrs []string
	for _, answer := range in.Answer {
		switch answer.(type) {
		case *dns.A:
			aRecord := answer.(*dns.A)
			addrs = append(addrs, aRecord.A.String())
		}
	}
	return addrs, nil
}
