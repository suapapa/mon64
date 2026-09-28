package mdns

import (
	"context"
	"net"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestParseAAnswer(t *testing.T) {
	name, err := dnsmessage.NewName("makit.local.")
	if err != nil {
		t.Fatal(err)
	}
	builder := dnsmessage.NewBuilder(nil, dnsmessage.Header{Response: true, Authoritative: true})
	if err := builder.StartAnswers(); err != nil {
		t.Fatal(err)
	}
	hdr := dnsmessage.ResourceHeader{
		Name:  name,
		Type:  dnsmessage.TypeA,
		Class: dnsmessage.ClassINET,
		TTL:   120,
	}
	if err := builder.AResource(hdr, dnsmessage.AResource{A: [4]byte{192, 168, 0, 42}}); err != nil {
		t.Fatal(err)
	}
	pkt, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}

	ip, ok := parseAAnswer(pkt, "makit.local")
	if !ok {
		t.Fatal("parseAAnswer failed")
	}
	if !ip.Equal(net.IPv4(192, 168, 0, 42)) {
		t.Fatalf("ip = %v", ip)
	}

	if _, ok := parseAAnswer(pkt, "other.local"); ok {
		t.Fatal("matched wrong name")
	}
}

func TestBuildAQuery(t *testing.T) {
	pkt, err := buildAQuery("makit.local")
	if err != nil {
		t.Fatal(err)
	}
	var p dnsmessage.Parser
	if _, err := p.Start(pkt); err != nil {
		t.Fatal(err)
	}
	q, err := p.Question()
	if err != nil {
		t.Fatal(err)
	}
	if q.Type != dnsmessage.TypeA {
		t.Fatalf("type = %v", q.Type)
	}
	if got := q.Name.String(); got != "makit.local." {
		t.Fatalf("name = %q", got)
	}
}

func TestLookupARejectsNonLocal(t *testing.T) {
	if _, err := LookupA(context.Background(), "example.com"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLookupALiveMakit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ip, err := LookupA(ctx, "makit.local")
	if err != nil {
		t.Skipf("makit.local not on LAN: %v", err)
	}
	if ip.To4() == nil {
		t.Fatalf("expected IPv4, got %v", ip)
	}
	t.Logf("makit.local -> %s", ip)
}
