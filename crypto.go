package main

import (
	"crypto/rc4"
	"net"
	"time"
)

// RC4Stream provides encryption/decryption functionality
type RC4Stream struct {
	cipher *rc4.Cipher
}

// NewRC4Stream creates a new RC4 stream cipher with the given key
func NewRC4Stream(key string) (*RC4Stream, error) {
	cipher, err := rc4.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}
	return &RC4Stream{cipher: cipher}, nil
}

// Encrypt encrypts data using RC4
func (r *RC4Stream) Encrypt(data []byte) {
	r.cipher.XORKeyStream(data, data)
}

// Decrypt decrypts data using RC4 (same as encrypt for RC4)
func (r *RC4Stream) Decrypt(data []byte) {
	r.cipher.XORKeyStream(data, data)
}

// RC4Conn wraps a connection with RC4 encryption
type RC4Conn struct {
	conn      net.Conn
	encStream *RC4Stream
	decStream *RC4Stream
}

// NewRC4Conn creates a new RC4-encrypted connection
func NewRC4Conn(conn net.Conn, key string) (*RC4Conn, error) {
	encStream, err := NewRC4Stream(key)
	if err != nil {
		return nil, err
	}

	decStream, err := NewRC4Stream(key)
	if err != nil {
		return nil, err
	}

	return &RC4Conn{
		conn:      conn,
		encStream: encStream,
		decStream: decStream,
	}, nil
}

// Read reads and decrypts data
func (rc *RC4Conn) Read(p []byte) (n int, err error) {
	n, err = rc.conn.Read(p)
	if err != nil {
		return n, err
	}

	rc.decStream.Decrypt(p[:n])
	return n, nil
}

// Write encrypts and writes data
func (rc *RC4Conn) Write(p []byte) (n int, err error) {
	encrypted := make([]byte, len(p))
	copy(encrypted, p)
	rc.encStream.Encrypt(encrypted)

	return rc.conn.Write(encrypted)
}

// Close closes the underlying connection
func (rc *RC4Conn) Close() error {
	return rc.conn.Close()
}

// LocalAddr returns the local network address
func (rc *RC4Conn) LocalAddr() net.Addr {
	return rc.conn.LocalAddr()
}

// RemoteAddr returns the remote network address
func (rc *RC4Conn) RemoteAddr() net.Addr {
	return rc.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines
func (rc *RC4Conn) SetDeadline(t time.Time) error {
	return rc.conn.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
func (rc *RC4Conn) SetReadDeadline(t time.Time) error {
	return rc.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls
func (rc *RC4Conn) SetWriteDeadline(t time.Time) error {
	return rc.conn.SetWriteDeadline(t)
}
