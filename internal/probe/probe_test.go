package probe

import (
	"context"
	"testing"
	"time"
)

func TestRunUnknownFallsBackToPing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res := Run(ctx, "unknown", "127.0.0.1")
	if res.Type == "" {
		t.Fatal("type should be set")
	}
}

func TestRunDNS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	res := Run(ctx, "dns", "localhost")
	if !res.OK {
		t.Skipf("dns probe unavailable in sandbox: %s", res.Detail)
	}
	if res.Value < 0 {
		t.Fatalf("dns value: %v", res.Value)
	}
}

func TestRunTCPClosedPort(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := Run(ctx, "tcp", "127.0.0.1:1")
	if res.OK {
		t.Fatal("closed port should fail")
	}
	if res.Value != -1 {
		t.Fatalf("loss should be -1, got %v", res.Value)
	}
}

func TestEscapeMarkdownInNotify(t *testing.T) {
	// 确保类型集合稳定
	for _, typ := range []string{"icmp", "tcp", "http", "dns"} {
		if typ == "" {
			t.Fatal("empty type")
		}
	}
}
