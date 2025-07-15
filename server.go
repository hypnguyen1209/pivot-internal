package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
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
	// Check if this is an agent connection mode (indicated by no colon in listenAddr for port)
	if s.isAgentMode() {
		return s.startAgentMode(ctx)
	}

	// Original listen mode
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

// isAgentMode determines if server should connect to agent
func (s *Server) isAgentMode() bool {
	// If listenAddr doesn't start with ":", it's an agent address
	return s.listenAddr != "" && s.listenAddr[0] != ':'
}

// startAgentMode connects to agent server and starts local SOCKS5 server for agent connections
func (s *Server) startAgentMode(ctx context.Context) error {
	agentAddr := s.listenAddr // Using listenAddr field to store agent address

	log.Printf("Server connecting to agent at %s", agentAddr)

	// Start local SOCKS5 server on port 9999 for agent connections only
	go func() {
		listener, err := net.Listen("tcp", ":9999")
		if err != nil {
			log.Printf("Failed to start local SOCKS5 server: %v", err)
			return
		}
		defer listener.Close()

		log.Printf("Started local SOCKS5 server on :9999 for agent connections")

		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			s.wg.Add(1)
			go func(conn net.Conn) {
				defer s.wg.Done()
				defer conn.Close()

				connID := atomic.AddInt32(&s.connCount, 1)
				log.Printf("Local SOCKS5 connection #%d from agent", connID)

				// Create encrypted connection
				rc4Conn, err := NewRC4Conn(conn, s.key)
				if err != nil {
					log.Printf("Connection #%d: Failed to create RC4 connection: %v", connID, err)
					return
				}

				// Handle SOCKS5 protocol
				s.handleSOCKS5(rc4Conn, connID)
			}(conn)
		}
	}()

	// Keep control connection to agent alive
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.shutdown:
			return nil
		default:
		}

		// Connect to agent
		conn, err := net.Dial("tcp", agentAddr)
		if err != nil {
			log.Printf("Failed to connect to agent: %v, retrying in 5 seconds...", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-s.shutdown:
				return nil
			case <-time.After(5 * time.Second):
				continue
			}
		}

		log.Printf("Connected to agent server at %s", agentAddr)

		// Create encrypted connection to agent
		rc4Conn, err := NewRC4Conn(conn, s.key)
		if err != nil {
			log.Printf("Failed to create RC4 connection: %v", err)
			conn.Close()
			continue
		}

		log.Printf("Established encrypted control connection to agent")

		// Keep the control connection alive
		buffer := make([]byte, 1)
		for {
			select {
			case <-ctx.Done():
				rc4Conn.Close()
				return ctx.Err()
			case <-s.shutdown:
				rc4Conn.Close()
				return nil
			default:
			}

			// Keep connection alive with periodic reads
			rc4Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			_, err := rc4Conn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Control connection to agent lost: %v, reconnecting...", err)
				rc4Conn.Close()
				break
			}
		}
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
	targetConn, err := s.handleSOCKS5Connect(conn)
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

func (s *Server) handleSOCKS5Connect(conn net.Conn) (net.Conn, error) {
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
