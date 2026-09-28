// Package mdns resolves .local hostnames via Multicast DNS without cgo.
//
// It sends a legacy (non-5353 source port) A query to 224.0.0.251:5353 and
// accepts the unicast response (RFC 6762 §6.7).
package mdns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

const (
	mdnsIPv4   = "224.0.0.251:5353"
	maxPkt     = 9000
	retransmit = 500 * time.Millisecond
)

// LookupA resolves name (e.g. "makit.local") to an IPv4 address over mDNS.
func LookupA(ctx context.Context, name string) (net.IP, error) {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	if name == "" {
		return nil, fmt.Errorf("mDNS: empty name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".local") {
		return nil, fmt.Errorf("mDNS: %q is not a .local name", name)
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
	}

	pkt, err := buildAQuery(name)
	if err != nil {
		return nil, err
	}

	c, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		return nil, fmt.Errorf("mDNS listen: %w", err)
	}
	defer c.Close()

	dst, err := net.ResolveUDPAddr("udp4", mdnsIPv4)
	if err != nil {
		return nil, err
	}
	send := func() error {
		_, err := c.WriteTo(pkt, dst)
		return err
	}
	if err := send(); err != nil {
		return nil, fmt.Errorf("mDNS query %s: %w", name, err)
	}

	want := strings.ToLower(name)
	buf := make([]byte, maxPkt)
	nextSend := time.Now().Add(retransmit)

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("mDNS lookup %s: %w", name, ctx.Err())
		default:
		}

		deadline := nextSend
		if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}
		_ = c.SetReadDeadline(deadline)

		n, _, err := c.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("mDNS lookup %s: %w", name, ctx.Err())
				default:
				}
				if err := send(); err != nil {
					return nil, fmt.Errorf("mDNS query %s: %w", name, err)
				}
				nextSend = time.Now().Add(retransmit)
				continue
			}
			return nil, fmt.Errorf("mDNS lookup %s: %w", name, err)
		}

		if ip, ok := parseAAnswer(buf[:n], want); ok {
			return ip, nil
		}
	}
}

func buildAQuery(name string) ([]byte, error) {
	fqdn := name + "."
	msgName, err := dnsmessage.NewName(fqdn)
	if err != nil {
		return nil, fmt.Errorf("mDNS name %q: %w", name, err)
	}
	builder := dnsmessage.NewBuilder(make([]byte, 0, 512), dnsmessage.Header{})
	if err := builder.StartQuestions(); err != nil {
		return nil, err
	}
	if err := builder.Question(dnsmessage.Question{
		Name:  msgName,
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
	}); err != nil {
		return nil, err
	}
	return builder.Finish()
}

func parseAAnswer(pkt []byte, wantHostLower string) (net.IP, bool) {
	var p dnsmessage.Parser
	if _, err := p.Start(pkt); err != nil {
		return nil, false
	}
	if err := p.SkipAllQuestions(); err != nil {
		return nil, false
	}
	for {
		rh, err := p.AnswerHeader()
		if err != nil {
			return nil, false
		}
		if rh.Type != dnsmessage.TypeA {
			if err := p.SkipAnswer(); err != nil {
				return nil, false
			}
			continue
		}
		a, err := p.AResource()
		if err != nil {
			return nil, false
		}
		got := strings.TrimSuffix(strings.ToLower(rh.Name.String()), ".")
		if got != wantHostLower {
			continue
		}
		ip := net.IP(a.A[:]).To4()
		if ip == nil {
			continue
		}
		return ip, true
	}
}
