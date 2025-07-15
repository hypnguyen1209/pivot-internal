package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
)

const (
	SOCKS5_VERSION = 0x05
	SOCKS5_CONNECT = 0x01
	SOCKS5_IPV4    = 0x01
	SOCKS5_DOMAIN  = 0x03
	SOCKS5_IPV6    = 0x04
)

// SOCKS5Server implements a SOCKS5 proxy server
type SOCKS5Server struct {
	listener net.Listener
}

// NewSOCKS5Server creates a new SOCKS5 server
func NewSOCKS5Server(addr string) (*SOCKS5Server, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &SOCKS5Server{listener: listener}, nil
}

// Start starts the SOCKS5 server
func (s *SOCKS5Server) Start() error {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}

		go s.handleConnection(conn)
	}
}

// Close closes the server
func (s *SOCKS5Server) Close() error {
	return s.listener.Close()
}

func (s *SOCKS5Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Step 1: Authentication negotiation
	if err := s.handleAuth(conn); err != nil {
		return
	}

	// Step 2: Handle CONNECT request
	targetConn, err := s.handleConnect(conn)
	if err != nil {
		return
	}
	defer targetConn.Close()

	// Step 3: Relay data
	relay(conn, targetConn)
}

func (s *SOCKS5Server) handleAuth(conn net.Conn) error {
	// Read authentication methods
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return err
	}

	if buf[0] != SOCKS5_VERSION {
		return fmt.Errorf("unsupported SOCKS version: %d", buf[0])
	}

	nMethods := buf[1]
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return err
	}

	// We support no authentication (0x00)
	response := []byte{SOCKS5_VERSION, 0x00}
	_, err := conn.Write(response)
	return err
}

func (s *SOCKS5Server) handleConnect(conn net.Conn) (net.Conn, error) {
	// Read CONNECT request
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}

	if buf[0] != SOCKS5_VERSION || buf[1] != SOCKS5_CONNECT {
		return nil, fmt.Errorf("unsupported request")
	}

	// Parse target address
	var targetAddr string
	var err error

	switch buf[3] {
	case SOCKS5_IPV4:
		targetAddr, err = s.parseIPv4Address(conn)
	case SOCKS5_DOMAIN:
		targetAddr, err = s.parseDomainAddress(conn)
	case SOCKS5_IPV6:
		targetAddr, err = s.parseIPv6Address(conn)
	default:
		return nil, fmt.Errorf("unsupported address type: %d", buf[3])
	}

	if err != nil {
		return nil, err
	}

	// Connect to target
	targetConn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		// Send error response
		response := []byte{SOCKS5_VERSION, 0x01, 0x00, SOCKS5_IPV4, 0, 0, 0, 0, 0, 0}
		conn.Write(response)
		return nil, err
	}

	// Send success response
	response := []byte{SOCKS5_VERSION, 0x00, 0x00, SOCKS5_IPV4, 0, 0, 0, 0, 0, 0}
	if _, err := conn.Write(response); err != nil {
		targetConn.Close()
		return nil, err
	}

	return targetConn, nil
}

func (s *SOCKS5Server) parseIPv4Address(conn net.Conn) (string, error) {
	buf := make([]byte, 6) // 4 bytes IP + 2 bytes port
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", err
	}

	ip := net.IP(buf[:4])
	port := binary.BigEndian.Uint16(buf[4:6])

	return fmt.Sprintf("%s:%d", ip.String(), port), nil
}

func (s *SOCKS5Server) parseDomainAddress(conn net.Conn) (string, error) {
	// Read domain length
	buf := make([]byte, 1)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", err
	}

	domainLen := buf[0]

	// Read domain + port
	buf = make([]byte, int(domainLen)+2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", err
	}

	domain := string(buf[:domainLen])
	port := binary.BigEndian.Uint16(buf[domainLen:])

	return fmt.Sprintf("%s:%d", domain, port), nil
}

func (s *SOCKS5Server) parseIPv6Address(conn net.Conn) (string, error) {
	buf := make([]byte, 18) // 16 bytes IP + 2 bytes port
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", err
	}

	ip := net.IP(buf[:16])
	port := binary.BigEndian.Uint16(buf[16:18])

	return fmt.Sprintf("[%s]:%d", ip.String(), port), nil
}

// SOCKS5Client provides client-side SOCKS5 functionality
type SOCKS5Client struct {
	proxyAddr string
}

// NewSOCKS5Client creates a new SOCKS5 client
func NewSOCKS5Client(proxyAddr string) *SOCKS5Client {
	return &SOCKS5Client{proxyAddr: proxyAddr}
}

// Connect connects to a target through the SOCKS5 proxy
func (c *SOCKS5Client) Connect(targetAddr string) (net.Conn, error) {
	// Connect to proxy
	conn, err := net.Dial("tcp", c.proxyAddr)
	if err != nil {
		return nil, err
	}

	// Perform SOCKS5 handshake
	if err := c.handshake(conn, targetAddr); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (c *SOCKS5Client) handshake(conn net.Conn, targetAddr string) error {
	// Step 1: Authentication
	if err := c.authenticate(conn); err != nil {
		return err
	}

	// Step 2: Connect request
	return c.connectRequest(conn, targetAddr)
}

func (c *SOCKS5Client) authenticate(conn net.Conn) error {
	// Send authentication methods (no auth)
	request := []byte{SOCKS5_VERSION, 0x01, 0x00}
	if _, err := conn.Write(request); err != nil {
		return err
	}

	// Read response
	response := make([]byte, 2)
	if _, err := io.ReadFull(conn, response); err != nil {
		return err
	}

	if response[0] != SOCKS5_VERSION || response[1] != 0x00 {
		return fmt.Errorf("authentication failed")
	}

	return nil
}

func (c *SOCKS5Client) connectRequest(conn net.Conn, targetAddr string) error {
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}

	// Build connect request
	request := []byte{SOCKS5_VERSION, SOCKS5_CONNECT, 0x00}

	// Add address
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			// IPv4
			request = append(request, SOCKS5_IPV4)
			request = append(request, ip4...)
		} else {
			// IPv6
			request = append(request, SOCKS5_IPV6)
			request = append(request, ip.To16()...)
		}
	} else {
		// Domain
		request = append(request, SOCKS5_DOMAIN)
		request = append(request, byte(len(host)))
		request = append(request, []byte(host)...)
	}

	// Add port
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	request = append(request, portBytes...)

	// Send request
	if _, err := conn.Write(request); err != nil {
		return err
	}

	// Read response
	response := make([]byte, 4)
	if _, err := io.ReadFull(conn, response); err != nil {
		return err
	}

	if response[0] != SOCKS5_VERSION || response[1] != 0x00 {
		return fmt.Errorf("connect failed, code: %d", response[1])
	}

	// Read remaining response based on address type
	switch response[3] {
	case SOCKS5_IPV4:
		remaining := make([]byte, 6)
		_, err = io.ReadFull(conn, remaining)
	case SOCKS5_DOMAIN:
		lenBuf := make([]byte, 1)
		if _, err = io.ReadFull(conn, lenBuf); err != nil {
			return err
		}
		remaining := make([]byte, int(lenBuf[0])+2)
		_, err = io.ReadFull(conn, remaining)
	case SOCKS5_IPV6:
		remaining := make([]byte, 18)
		_, err = io.ReadFull(conn, remaining)
	}

	return err
}

// relay copies data between two connections
func relay(conn1, conn2 net.Conn) {
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(conn1, conn2)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(conn2, conn1)
		done <- struct{}{}
	}()

	<-done
}
