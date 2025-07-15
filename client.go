package main

import (
	"context"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

// Client represents the pivot client
type Client struct {
	key        string
	remoteAddr string
	localAddr  string
	server     *SOCKS5Server
	wg         sync.WaitGroup
	shutdown   chan struct{}
	connCount  int32
}

// NewClient creates a new client instance
func NewClient(key, remoteAddr, localAddr string) *Client {
	return &Client{
		key:        key,
		remoteAddr: remoteAddr,
		localAddr:  localAddr,
		shutdown:   make(chan struct{}),
	}
}

// Start starts the client
func (c *Client) Start(ctx context.Context) error {
	// Start local SOCKS5 server
	socks5Server, err := NewSOCKS5Server(c.localAddr)
	if err != nil {
		return err
	}
	c.server = socks5Server

	log.Printf("Client SOCKS5 server listening on %s", c.localAddr)
	log.Printf("Will forward to remote server at %s", c.remoteAddr)

	// Accept connections in a goroutine
	go func() {
		defer socks5Server.Close()
		for {
			select {
			case <-c.shutdown:
				return
			default:
			}

			// Accept local SOCKS5 connections
			localConn, err := socks5Server.listener.Accept()
			if err != nil {
				select {
				case <-c.shutdown:
					return
				default:
					log.Printf("Accept error: %v", err)
					continue
				}
			}

			c.wg.Add(1)
			go func(conn net.Conn) {
				defer c.wg.Done()
				c.handleLocalConnection(conn)
			}(localConn)
		}
	}()

	// Wait for shutdown signal
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.shutdown:
		return nil
	}
}

func (c *Client) handleLocalConnection(localConn net.Conn) {
	defer localConn.Close()

	connID := atomic.AddInt32(&c.connCount, 1)
	log.Printf("New local SOCKS5 connection #%d from %s", connID, localConn.RemoteAddr())

	// Connect to remote server
	remoteConn, err := net.Dial("tcp", c.remoteAddr)
	if err != nil {
		log.Printf("Connection #%d: Failed to connect to remote server: %v", connID, err)
		return
	}
	defer remoteConn.Close()

	log.Printf("Connection #%d: Connected to remote server %s", connID, c.remoteAddr)

	// Wrap remote connection with RC4 encryption
	rc4Conn, err := NewRC4Conn(remoteConn, c.key)
	if err != nil {
		log.Printf("Connection #%d: Failed to create RC4 connection: %v", connID, err)
		return
	}

	log.Printf("Connection #%d: Starting relay", connID)

	// Start relaying all data between local and remote
	relay(localConn, rc4Conn)
	log.Printf("Connection #%d: Closed", connID)
}

// Shutdown gracefully shuts down the client
func (c *Client) Shutdown(ctx context.Context) error {
	log.Printf("Shutting down client...")

	// Signal shutdown
	close(c.shutdown)

	// Close the server listener
	if c.server != nil {
		c.server.Close()
	}

	// Wait for all connections to finish or timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("All client connections closed")
		return nil
	case <-ctx.Done():
		log.Printf("Shutdown timeout reached, forcing close")
		return ctx.Err()
	}
}
