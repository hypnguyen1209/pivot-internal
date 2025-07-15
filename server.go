package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

// Server represents the pivot server
type Server struct {
	key        string
	listenAddr string
	listener   net.Listener
	wg         sync.WaitGroup
	shutdown   chan struct{}
	connCount  int32
}

// NewServer creates a new server instance
func NewServer(key, listenAddr string) *Server {
	return &Server{
		key:        key,
		listenAddr: listenAddr,
		shutdown:   make(chan struct{}),
	}
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	s.listener = listener

	log.Printf("Server listening on %s", s.listenAddr)

	// Accept connections in a goroutine
	go func() {
		defer listener.Close()
		for {
			select {
			case <-s.shutdown:
				return
			default:
			}

			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-s.shutdown:
					return
				default:
					log.Printf("Accept error: %v", err)
					continue
				}
			}

			s.wg.Add(1)
			go func(conn net.Conn) {
				defer s.wg.Done()
				s.handleClient(conn)
			}(conn)
		}
	}()

	// Wait for shutdown signal
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.shutdown:
		return nil
	}
}

func (s *Server) handleClient(clientConn net.Conn) {
	defer clientConn.Close()

	connID := atomic.AddInt32(&s.connCount, 1)

	// Wrap connection with RC4 encryption
	rc4Conn, err := NewRC4Conn(clientConn, s.key)
	if err != nil {
		log.Printf("Connection #%d: Failed to create RC4 connection: %v", connID, err)
		return
	}

	// Create SOCKS5 server for this client
	s.handleSOCKS5(rc4Conn, connID)
}

func (s *Server) handleSOCKS5(conn net.Conn, connID int32) {
	// Implement SOCKS5 protocol handling

	// Step 1: Authentication negotiation
	if err := s.handleSOCKS5Auth(conn); err != nil {
		log.Printf("Connection #%d: SOCKS5 auth error: %v", connID, err)
		return
	}

	// Step 2: Handle CONNECT request
	targetConn, err := s.handleSOCKS5Connect(conn, connID)
	if err != nil {
		log.Printf("Connection #%d: SOCKS5 connect error: %v", connID, err)
		return
	}
	defer targetConn.Close()

	// Step 3: Relay data
	relay(conn, targetConn)
}

func (s *Server) handleSOCKS5Auth(conn net.Conn) error {
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

	// Send no authentication required
	response := []byte{SOCKS5_VERSION, 0x00}
	_, err := conn.Write(response)
	return err
}

func (s *Server) handleSOCKS5Connect(conn net.Conn, connID int32) (net.Conn, error) {
	// Read CONNECT request header
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}

	if buf[0] != SOCKS5_VERSION || buf[1] != SOCKS5_CONNECT {
		return nil, fmt.Errorf("unsupported request")
	}

	// Parse target address based on address type
	var targetAddr string
	var err error

	switch buf[3] {
	case SOCKS5_IPV4:
		targetAddr, err = s.parseIPv4(conn)
	case SOCKS5_DOMAIN:
		targetAddr, err = s.parseDomain(conn)
	case SOCKS5_IPV6:
		targetAddr, err = s.parseIPv6(conn)
	default:
		// Send error response
		response := []byte{SOCKS5_VERSION, 0x08, 0x00, SOCKS5_IPV4, 0, 0, 0, 0, 0, 0}
		conn.Write(response)
		return nil, fmt.Errorf("unsupported address type: %d", buf[3])
	}

	if err != nil {
		// Send error response
		response := []byte{SOCKS5_VERSION, 0x01, 0x00, SOCKS5_IPV4, 0, 0, 0, 0, 0, 0}
		conn.Write(response)
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

func (s *Server) parseIPv4(conn net.Conn) (string, error) {
	buf := make([]byte, 6) // 4 bytes IP + 2 bytes port
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", err
	}

	ip := net.IP(buf[:4])
	port := uint16(buf[4])<<8 | uint16(buf[5])

	return fmt.Sprintf("%s:%d", ip.String(), port), nil
}

func (s *Server) parseDomain(conn net.Conn) (string, error) {
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
	port := uint16(buf[domainLen])<<8 | uint16(buf[domainLen+1])

	return fmt.Sprintf("%s:%d", domain, port), nil
}

func (s *Server) parseIPv6(conn net.Conn) (string, error) {
	buf := make([]byte, 18) // 16 bytes IP + 2 bytes port
	if _, err := io.ReadFull(conn, buf); err != nil {
		return "", err
	}

	ip := net.IP(buf[:16])
	port := uint16(buf[16])<<8 | uint16(buf[17])

	return fmt.Sprintf("[%s]:%d", ip.String(), port), nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	close(s.shutdown)

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all connections to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All connections closed")
		return nil
	case <-ctx.Done():
		log.Println("Shutdown timeout reached, forcing close")
		return ctx.Err()
	}
}
