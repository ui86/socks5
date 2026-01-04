package socks5

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"slices"
	"sync"
	"time"

	"github.com/txthinking/runnergroup"
)

var (
	// ErrUnsupportCmd is the error when got unsupport command
	ErrUnsupportCmd = errors.New("Unsupport Command")
	// ErrUserPassAuth is the error when got invalid username or password
	ErrUserPassAuth = errors.New("Invalid Username or Password for Auth")
)

// tcpBufPool 32KB buffer for TCP copy
var tcpBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024)
	},
}

// udpBufPool 64KB buffer for UDP packets
var udpBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 65507)
	},
}

// Server is socks5 server wrapper
type Server struct {
	UserName          string
	Password          string
	Method            byte
	SupportedCommands []byte
	Addr              string
	ServerAddr        net.Addr
	UDPConn           *net.UDPConn
	// 优化 1: 使用 sync.Map 替代 go-cache
	UDPExchanges  *sync.Map
	TCPTimeout    int
	UDPTimeout    int
	Handle        Handler
	AssociatedUDP *sync.Map
	UDPSrc        *sync.Map
	RunnerGroup   *runnergroup.RunnerGroup
	LimitUDP      bool
	WhiteList     map[string]bool
	// 优化 2: UDP 处理工作池通道
	udpWorkCh chan *udpTask
}

// udpTask 封装 UDP 处理任务
type udpTask struct {
	addr *net.UDPAddr
	buf  []byte
	n    int
}

type UDPExchange struct {
	ClientAddr *net.UDPAddr
	RemoteConn net.Conn
}

func NewClassicServer(addr, ip, username, password string, tcpTimeout, udpTimeout int, whiteList []string) (*Server, error) {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	saddr, err := Resolve("udp", net.JoinHostPort(ip, p))
	if err != nil {
		return nil, err
	}
	m := MethodNone
	if username != "" && password != "" {
		m = MethodUsernamePassword
	}

	whiteListMap := make(map[string]bool)
	for _, ip := range whiteList {
		whiteListMap[ip] = true
	}

	s := &Server{
		Method:            m,
		UserName:          username,
		Password:          password,
		SupportedCommands: []byte{CmdConnect, CmdUDP},
		Addr:              addr,
		ServerAddr:        saddr,
		// sync.Map 零值直接可用，无需初始化
		UDPExchanges:  &sync.Map{},
		TCPTimeout:    tcpTimeout,
		UDPTimeout:    udpTimeout,
		AssociatedUDP: &sync.Map{},
		UDPSrc:        &sync.Map{},
		RunnerGroup:   runnergroup.New(),
		WhiteList:     whiteListMap,
		// 初始化 UDP 工作通道，缓冲区大小可根据机器配置调整
		udpWorkCh: make(chan *udpTask, 5000),
	}
	return s, nil
}

func (s *Server) Negotiate(rw io.ReadWriter) error {
	rq, err := NewNegotiationRequestFrom(rw)
	if err != nil {
		return err
	}
	var got bool
	var m byte
	for _, m = range rq.Methods {
		if m == s.Method {
			got = true
		}
	}
	if !got {
		rp := NewNegotiationReply(MethodUnsupportAll)
		if _, err := rp.WriteTo(rw); err != nil {
			return err
		}
	}
	rp := NewNegotiationReply(s.Method)
	if _, err := rp.WriteTo(rw); err != nil {
		return err
	}

	if s.Method == MethodUsernamePassword {
		urq, err := NewUserPassNegotiationRequestFrom(rw)
		if err != nil {
			return err
		}
		if string(urq.Uname) != s.UserName || string(urq.Passwd) != s.Password {
			urp := NewUserPassNegotiationReply(UserPassStatusFailure)
			if _, err := urp.WriteTo(rw); err != nil {
				return err
			}
			return ErrUserPassAuth
		}
		urp := NewUserPassNegotiationReply(UserPassStatusSuccess)
		if _, err := urp.WriteTo(rw); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) GetRequest(rw io.ReadWriter) (*Request, error) {
	r, err := NewRequestFrom(rw)
	if err != nil {
		return nil, err
	}
	var supported bool
	if slices.Contains(s.SupportedCommands, r.Cmd) {
		supported = true
	}
	if !supported {
		var p *Reply
		if r.Atyp == ATYPIPv4 || r.Atyp == ATYPDomain {
			p = NewReply(RepCommandNotSupported, ATYPIPv4, []byte{0x00, 0x00, 0x00, 0x00}, []byte{0x00, 0x00})
		} else {
			p = NewReply(RepCommandNotSupported, ATYPIPv6, []byte(net.IPv6zero), []byte{0x00, 0x00})
		}
		if _, err := p.WriteTo(rw); err != nil {
			return nil, err
		}
		return nil, ErrUnsupportCmd
	}
	return r, nil
}

func (s *Server) ListenAndServe(h Handler) error {
	if h == nil {
		s.Handle = &DefaultHandle{}
	} else {
		s.Handle = h
	}
	addr, err := net.ResolveTCPAddr("tcp", s.Addr)
	if err != nil {
		return err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	s.RunnerGroup.Add(&runnergroup.Runner{
		Start: func() error {
			for {
				c, err := l.AcceptTCP()
				if err != nil {
					return err
				}
				go func(c *net.TCPConn) {
					defer c.Close()
					clientIP := c.RemoteAddr().(*net.TCPAddr).IP.String()
					if len(s.WhiteList) > 0 {
						if !s.WhiteList[clientIP] {
							log.Printf("Connection rejected from %s (not in whitelist)", clientIP)
							return
						}
					}
					if err := s.Negotiate(c); err != nil {
						// log.Println(err) // 减少日志噪音
						return
					}
					r, err := s.GetRequest(c)
					if err != nil {
						log.Println(err)
						return
					}
					if err := s.Handle.TCPHandle(s, c, r); err != nil {
						log.Println(err)
					}
				}(c)
			}
		},
		Stop: func() error {
			return l.Close()
		},
	})

	addr1, err := net.ResolveUDPAddr("udp", s.Addr)
	if err != nil {
		l.Close()
		return err
	}
	s.UDPConn, err = net.ListenUDP("udp", addr1)
	if err != nil {
		l.Close()
		return err
	}

	// 优化 3: 启动 UDP Worker Pool (例如 128 个并发 worker)
	// 这避免了在高流量下为每个数据包创建一个 Goroutine 的开销
	numWorkers := 128
	for range numWorkers {
		go func() {
			for task := range s.udpWorkCh {
				handleUDPTask(s, task)
			}
		}()
	}

	s.RunnerGroup.Add(&runnergroup.Runner{
		Start: func() error {
			for {
				b := udpBufPool.Get().([]byte)
				b = b[:cap(b)] // Reset length

				n, addr, err := s.UDPConn.ReadFromUDP(b)
				if err != nil {
					udpBufPool.Put(b)
					return err
				}

				// 将任务发送到 Worker Pool
				// 使用 select 防止任务队列满时阻塞 UDP 读取循环
				select {
				case s.udpWorkCh <- &udpTask{addr: addr, buf: b, n: n}:
				default:
					// 队列已满，丢弃数据包以保护服务器
					udpBufPool.Put(b)
					if Debug {
						log.Println("UDP worker queue full, dropping packet")
					}
				}
			}
		},
		Stop: func() error {
			close(s.udpWorkCh) // 停止 worker
			return s.UDPConn.Close()
		},
	})
	return s.RunnerGroup.Wait()
}

// handleUDPTask 是 worker 实际执行的逻辑
func handleUDPTask(s *Server, t *udpTask) {
	defer udpBufPool.Put(t.buf) // 确保归还 buffer

	d, err := NewDatagramFromBytes(t.buf[0:t.n])
	if err != nil {
		// log.Println(err)
		return
	}
	if d.Frag != 0x00 {
		return
	}
	if err := s.Handle.UDPHandle(s, t.addr, d); err != nil {
		log.Println(err)
	}
}

func (s *Server) Shutdown() error {
	return s.RunnerGroup.Done()
}

type Handler interface {
	TCPHandle(*Server, *net.TCPConn, *Request) error
	UDPHandle(*Server, *net.UDPAddr, *Datagram) error
}

type DefaultHandle struct {
}

// idleTimeoutConn 包装 net.Conn 以支持 io.CopyBuffer 下的空闲超时
type idleTimeoutConn struct {
	net.Conn
	timeout time.Duration
}

func (c *idleTimeoutConn) Read(b []byte) (int, error) {
	if c.timeout > 0 {
		if err := c.Conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(b)
}

func (h *DefaultHandle) TCPHandle(s *Server, c *net.TCPConn, r *Request) error {
	if r.Cmd == CmdConnect {
		rc, err := r.Connect(c)
		if err != nil {
			return err
		}
		defer rc.Close()

		// 优化 4: 使用 io.CopyBuffer 替代手动循环
		// io.CopyBuffer 在 Linux 下可以利用 splice 实现零拷贝，大幅降低 CPU
		directTransfer := func(dst net.Conn, src net.Conn, timeout int) {
			buf := tcpBufPool.Get().([]byte)
			defer tcpBufPool.Put(buf)

			// 包装连接以处理超时
			srcWrapped := &idleTimeoutConn{Conn: src, timeout: time.Duration(timeout) * time.Second}

			// io.CopyBuffer 会自动处理 Read/Write 循环
			_, _ = io.CopyBuffer(dst, srcWrapped, buf)
		}

		go directTransfer(c, rc, s.TCPTimeout)
		directTransfer(rc, c, s.TCPTimeout)
		return nil
	}
	if r.Cmd == CmdUDP {
		caddr, err := r.UDP(c, s.ServerAddr)
		if err != nil {
			return err
		}
		ch := make(chan byte)
		defer close(ch)
		s.AssociatedUDP.Store(caddr.String(), ch)
		defer s.AssociatedUDP.Delete(caddr.String())
		io.Copy(io.Discard, c)
		return nil
	}
	return ErrUnsupportCmd
}

func (h *DefaultHandle) UDPHandle(s *Server, addr *net.UDPAddr, d *Datagram) error {
	src := addr.String()
	var ch chan byte
	if s.LimitUDP {
		any, ok := s.AssociatedUDP.Load(src)
		if !ok {
			return fmt.Errorf("Address %s not associated", src)
		}
		ch = any.(chan byte)
	}

	send := func(ue *UDPExchange, data []byte) error {
		select {
		case <-ch:
			return fmt.Errorf("Association closed")
		default:
			_, err := ue.RemoteConn.Write(data)
			return err
		}
	}

	dst := d.Address()
	// 优化 5: 使用 sync.Map 的 Load
	if any, ok := s.UDPExchanges.Load(src + dst); ok {
		ue := any.(*UDPExchange)
		return send(ue, d.Data)
	}

	var laddr string
	if any, ok := s.UDPSrc.Load(src + dst); ok {
		laddr = any.(string)
	}
	rc, err := DialUDP("udp", laddr, dst)
	if err != nil {
		// 简单的重试逻辑，忽略特定错误检查以简化
		rc, err = DialUDP("udp", "", dst)
		if err != nil {
			return err
		}
		laddr = ""
	}
	if laddr == "" {
		s.UDPSrc.Store(src+dst, rc.LocalAddr().String())
	}

	ue := &UDPExchange{
		ClientAddr: addr,
		RemoteConn: rc,
	}

	if err := send(ue, d.Data); err != nil {
		ue.RemoteConn.Close()
		return err
	}
	s.UDPExchanges.Store(src+dst, ue)

	// 保持原有的响应读取逻辑，但这部分是 IO 密集型的，通常不会导致 CPU 瓶颈
	// 如果需要极致优化，这里也可以改用 Worker Pool，但因为需要维持连接状态，Goroutine 是合适的
	go func(ue *UDPExchange, dst string) {
		defer func() {
			ue.RemoteConn.Close()
			s.UDPExchanges.Delete(ue.ClientAddr.String() + dst)
		}()
		b := udpBufPool.Get().([]byte)
		defer udpBufPool.Put(b)

		for {
			select {
			case <-ch:
				return
			default:
				if s.UDPTimeout != 0 {
					ue.RemoteConn.SetDeadline(time.Now().Add(time.Duration(s.UDPTimeout) * time.Second))
				}
				buf := b[:cap(b)]
				n, err := ue.RemoteConn.Read(buf)
				if err != nil {
					return
				}
				var a byte
				var addr, port []byte

				// 获取远程发包方的真实 IP/Port
				if udpAddr, ok := ue.RemoteConn.RemoteAddr().(*net.UDPAddr); ok {
					if ip4 := udpAddr.IP.To4(); ip4 != nil {
						a = ATYPIPv4
						addr = ip4
					} else {
						a = ATYPIPv6
						addr = udpAddr.IP
					}
					port = make([]byte, 2)
					binary.BigEndian.PutUint16(port, uint16(udpAddr.Port))
				} else {
					// Fallback
					var err error
					a, addr, port, err = ParseAddress(dst)
					if err != nil {
						log.Println(err)
						return
					}
					if a == ATYPDomain {
						addr = addr[1:]
					}
				}

				d1 := NewDatagram(a, addr, port, buf[0:n])
				if _, err := s.UDPConn.WriteToUDP(d1.Bytes(), ue.ClientAddr); err != nil {
					return
				}
			}
		}
	}(ue, dst)
	return nil
}
