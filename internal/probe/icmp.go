package probe

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ICMPPing 发送 ICMP Echo 并测量 RTT（毫秒）。
// 需要 CAP_NET_RAW 或 root；失败时调用方回退 TCPPing。
func ICMPPing(ctx context.Context, target string) (float64, error) {
	ip := net.ParseIP(target)
	if ip == nil {
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", target)
		if err != nil || len(ips) == 0 {
			return 0, fmt.Errorf("resolve %s: %w", target, err)
		}
		ip = ips[0]
	}
	// 优先非特权 ICMP（Linux net.ipv4.ping_group_range 允许）
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	}
	if err != nil {
		return 0, fmt.Errorf("icmp listen: %w", err)
	}
	defer conn.Close()

	pid := os.Getpid() & 0xffff
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{ID: pid, Seq: 1, Data: []byte("sounding")},
	}
	payload, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	dst := &net.UDPAddr{IP: ip}
	if _, err := conn.WriteTo(payload, dst); err != nil {
		return 0, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	reply := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		n, _, err := conn.ReadFrom(reply)
		if err != nil {
			return 0, err
		}
		rm, err := icmp.ParseMessage(ipv4.ICMPTypeEcho.Protocol(), reply[:n])
		if err != nil {
			continue
		}
		if rm.Type == ipv4.ICMPTypeEchoReply {
			return float64(time.Since(start).Microseconds()) / 1000, nil
		}
	}
}

// Ping 优先 ICMP，失败回退 TCP（返回毫秒与使用的方法）。
func Ping(ctx context.Context, target string) (float64, string, error) {
	host := target
	if h, _, err := net.SplitHostPort(target); err == nil {
		host = h
	}
	if ms, err := ICMPPing(ctx, host); err == nil {
		return ms, "icmp", nil
	}
	tcpTarget := target
	if _, _, err := net.SplitHostPort(tcpTarget); err != nil {
		tcpTarget = net.JoinHostPort(host, "443")
	}
	ms, err := TCPPing(ctx, tcpTarget)
	return ms, "tcp", err
}

var _ = binary.BigEndian
