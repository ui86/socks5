package socks5

import (
	"net"
	"time"
)

var Debug bool

func init() {
	// log.SetFlags(log.LstdFlags | log.Lshortfile)
}

var Resolve func(network string, addr string) (net.Addr, error) = func(network string, addr string) (net.Addr, error) {
	if network == "tcp" {
		return net.ResolveTCPAddr("tcp", addr)
	}
	return net.ResolveUDPAddr("udp", addr)
}

// 优化 1: 使用 net.Dialer 替代原本的分步 Resolve + Dial
// 优势:
// 1. 启用 "Happy Eyeballs" (并发尝试 IPv4/IPv6)，减少连接延迟
// 2. 内置超时控制 (Timeout) 和 KeepAlive
var DialTCP func(network string, laddr, raddr string) (net.Conn, error) = func(network string, laddr, raddr string) (net.Conn, error) {
	// 创建 Dialer 实例
	dialer := &net.Dialer{
		Timeout:   10 * time.Second, // 默认连接超时，防止长时间挂起
		KeepAlive: 30 * time.Second, // 保持连接活跃
	}

	// 如果指定了本地地址
	if laddr != "" {
		local, err := net.ResolveTCPAddr(network, laddr)
		if err == nil {
			dialer.LocalAddr = local
		}
	}

	// 直接 Dial，让 Go 标准库处理 DNS 解析和 IP 选择优化
	return dialer.Dial(network, raddr)
}

// DialUDP 优化: 直接 DialUDP，避免 ResolveUDPAddr 两次
var DialUDP func(network string, laddr, raddr string) (net.Conn, error) = func(network string, laddr, raddr string) (net.Conn, error) {
	var la, ra *net.UDPAddr
	var err error
	if laddr != "" {
		la, err = net.ResolveUDPAddr(network, laddr)
		if err != nil {
			return nil, err
		}
	}
	ra, err = net.ResolveUDPAddr(network, raddr)
	if err != nil {
		return nil, err
	}
	return net.DialUDP(network, la, ra)
}
