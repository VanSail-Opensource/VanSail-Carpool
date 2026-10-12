package iplookup

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

func diagnosticService(t *testing.T) *EnhancedService {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	queues := map[string]*upstreamqueue.Queue{}
	for _, key := range []string{"dns", "checks"} {
		q, err := upstreamqueue.New(upstreamqueue.Config{Name: key, Capacity: 16, Workers: 4})
		if err != nil {
			t.Fatal(err)
		}
		if err := q.Start(ctx); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := q.Shutdown(context.Background()); err != nil {
				t.Error(err)
			}
		})
		queues[key] = q
	}
	return NewEnhancedService(nil, queues, nil)
}

func diagnosticDNS(t *testing.T, rcode int) (string, *atomic.Int32) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	calls := new(atomic.Int32)
	server := &dns.Server{PacketConn: conn, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		calls.Add(1)
		r := new(dns.Msg)
		r.SetReply(req)
		r.Rcode = rcode
		if rcode == dns.RcodeSuccess && req.Question[0].Qtype == dns.TypeA && !strings.HasPrefix(req.Question[0].Name, "nodata.") {
			r.Answer = []dns.RR{&dns.A{Hdr: dns.RR_Header{Name: req.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 120}, A: net.ParseIP("8.8.8.8")}}
		}
		if rcode == dns.RcodeNameError {
			r.Ns = []dns.RR{&dns.SOA{Hdr: dns.RR_Header{Name: "example.net.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 120}, Minttl: 60}}
		}
		if err := w.WriteMsg(r); err != nil {
			t.Error(err)
		}
	})}
	go func() {
		if err := server.ActivateAndServe(); err != nil {
			t.Error(err)
		}
	}()
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	return conn.LocalAddr().String(), calls
}

func TestDNSFailoverAndNegativeCaching(t *testing.T) {
	s := diagnosticService(t)
	failed, _ := diagnosticDNS(t, dns.RcodeRefused)
	healthy, _ := diagnosticDNS(t, dns.RcodeSuccess)
	r := s.resolveDNS(context.Background(), "edge.example.net", []string{healthy, failed}, 0, false, false)
	if r.status != "success" || len(r.ips) != 1 || r.ips[0].String() != "8.8.8.8" {
		t.Fatalf("failover: %+v", r)
	}
	negative, calls := diagnosticDNS(t, dns.RcodeNameError)
	r = s.resolveDNS(context.Background(), "absent.example.net", []string{negative}, 0, false, false)
	if r.status != "no_address" || len(r.ips) != 0 || r.ttl != time.Minute {
		t.Fatalf("negative: %+v", r)
	}
	before := calls.Load()
	s.resolveDNS(context.Background(), "absent.example.net", []string{negative}, 0, false, false)
	if calls.Load() != before {
		t.Fatal("negative response not cached")
	}
}

func TestDNSNoDataAndResolverCooldown(t *testing.T) {
	s := diagnosticService(t)
	server, calls := diagnosticDNS(t, dns.RcodeSuccess)
	r := s.resolveDNS(context.Background(), "nodata.example.net", []string{server}, 0, false, false)
	if r.status != "no_address" || len(r.ips) != 0 || calls.Load() != 2 {
		t.Fatalf("NODATA: %+v calls=%d", r, calls.Load())
	}
	s.resolvers["cooling"] = resolverHealth{cooldown: time.Now().Add(time.Minute)}
	counts := map[string]int{}
	for range 6 {
		order := s.resolverOrder([]string{"cooling", "first", "second"})
		if len(order) != 2 {
			t.Fatal("cooldown not respected")
		}
		counts[order[0]]++
	}
	if counts["first"] != 3 || counts["second"] != 3 {
		t.Fatalf("unbalanced healthy selection: %+v", counts)
	}
}
