package makit

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/plgd-dev/go-coap/v3/message"
	"github.com/plgd-dev/go-coap/v3/message/codes"
	"github.com/plgd-dev/go-coap/v3/net/blockwise"
	"github.com/plgd-dev/go-coap/v3/options"
	"github.com/plgd-dev/go-coap/v3/udp"
	"github.com/plgd-dev/go-coap/v3/udp/client"

	"github.com/suapapa/mon64/internal/config"
	"github.com/suapapa/mon64/internal/mdns"
)

const (
	framePath  = "/frame"
	brightPath = "/brightness"
	putTimeout = 30 * time.Second
)

// frameClient pushes RGB888 frames and brightness to a makit64 panel.
type frameClient interface {
	PutFrame(ctx context.Context, rgb []byte) error
	PutBrightness(ctx context.Context, level uint8) error
	Close() error
}

type deviceClient struct {
	conn *client.Conn
}

func dialDevice(addr string) (frameClient, error) {
	normalized, err := config.NormalizeMakit64Addr(addr)
	if err != nil {
		return nil, fmt.Errorf("makit64 addr: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), putTimeout)
	defer cancel()
	dialAddr, err := resolveDialAddr(ctx, normalized)
	if err != nil {
		return nil, err
	}

	conn, err := udp.Dial(
		dialAddr,
		options.WithBlockwise(true, blockwise.SZX1024, putTimeout),
		options.WithErrors(func(error) {}),
	)
	if err != nil {
		return nil, fmt.Errorf("dial makit64 %s (via %s): %w", normalized, dialAddr, err)
	}
	return &deviceClient{conn: conn}, nil
}

// resolveDialAddr turns host:port into an IP:port when host ends in .local,
// using pure-Go mDNS so CGO_ENABLED=0 builds still work.
func resolveDialAddr(ctx context.Context, hostPort string) (string, error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", err
	}
	if ip := net.ParseIP(host); ip != nil {
		return hostPort, nil
	}
	if !strings.HasSuffix(strings.ToLower(host), ".local") {
		return hostPort, nil
	}
	ip, err := mdns.LookupA(ctx, host)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", host, err)
	}
	return net.JoinHostPort(ip.String(), port), nil
}

func (c *deviceClient) Close() error {
	return c.conn.Close()
}

func (c *deviceClient) PutFrame(ctx context.Context, rgb []byte) error {
	if len(rgb) != frameBytes {
		return fmt.Errorf("makit64 frame: want %d bytes, got %d", frameBytes, len(rgb))
	}
	ctx, cancel := context.WithTimeout(ctx, putTimeout)
	defer cancel()
	resp, err := c.conn.Put(ctx, framePath, message.AppOctets, bytes.NewReader(rgb))
	if err != nil {
		return fmt.Errorf("PUT %s: %w", framePath, err)
	}
	if resp.Code() != codes.Changed {
		return fmt.Errorf("PUT %s: unexpected code %v", framePath, resp.Code())
	}
	return nil
}

func (c *deviceClient) PutBrightness(ctx context.Context, level uint8) error {
	ctx, cancel := context.WithTimeout(ctx, putTimeout)
	defer cancel()
	payload := bytes.NewReader([]byte(strconv.FormatUint(uint64(level), 10)))
	resp, err := c.conn.Put(ctx, brightPath, message.TextPlain, payload)
	if err != nil {
		return fmt.Errorf("PUT %s: %w", brightPath, err)
	}
	if resp.Code() != codes.Changed {
		return fmt.Errorf("PUT %s: unexpected code %v", brightPath, resp.Code())
	}
	return nil
}
