package main

import (
	"crypto/rc4"
	"net"
	"testing"
	"time"
)

func TestRC4Stream(t *testing.T) {
	key := "testkey123"
	stream, err := NewRC4Stream(key)
	if err != nil {
		t.Fatalf("Failed to create RC4 stream: %v", err)
	}

	// Test data
	original := []byte("Hello, World! This is a test message.")
	encrypted := make([]byte, len(original))
	copy(encrypted, original)

	// Encrypt
	stream.Encrypt(encrypted)

	// Verify it's different
	if string(encrypted) == string(original) {
		t.Error("Encrypted data should be different from original")
	}

	// Create new stream for decryption (RC4 requires fresh stream)
	decStream, err := NewRC4Stream(key)
	if err != nil {
		t.Fatalf("Failed to create RC4 stream for decryption: %v", err)
	}

	// Decrypt
	decStream.Decrypt(encrypted)

	// Verify it matches original
	if string(encrypted) != string(original) {
		t.Errorf("Decrypted data doesn't match original. Got: %s, Expected: %s", string(encrypted), string(original))
	}
}

func TestRC4WithStandardLibrary(t *testing.T) {
	key := []byte("testkey123")

	// Test with standard library
	cipher1, err := rc4.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	cipher2, err := rc4.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("Test data for RC4 encryption")
	encrypted := make([]byte, len(data))
	decrypted := make([]byte, len(data))

	copy(encrypted, data)

	// Encrypt
	cipher1.XORKeyStream(encrypted, encrypted)

	// Decrypt
	cipher2.XORKeyStream(decrypted, encrypted)

	if string(decrypted) != string(data) {
		t.Errorf("RC4 encryption/decryption failed. Got: %s, Expected: %s", string(decrypted), string(data))
	}
}

func TestSOCKS5Constants(t *testing.T) {
	if SOCKS5_VERSION != 0x05 {
		t.Errorf("SOCKS5_VERSION should be 0x05, got: 0x%02x", SOCKS5_VERSION)
	}

	if SOCKS5_CONNECT != 0x01 {
		t.Errorf("SOCKS5_CONNECT should be 0x01, got: 0x%02x", SOCKS5_CONNECT)
	}

	if SOCKS5_IPV4 != 0x01 {
		t.Errorf("SOCKS5_IPV4 should be 0x01, got: 0x%02x", SOCKS5_IPV4)
	}

	if SOCKS5_DOMAIN != 0x03 {
		t.Errorf("SOCKS5_DOMAIN should be 0x03, got: 0x%02x", SOCKS5_DOMAIN)
	}

	if SOCKS5_IPV6 != 0x04 {
		t.Errorf("SOCKS5_IPV6 should be 0x04, got: 0x%02x", SOCKS5_IPV6)
	}
}

// Test helper function to create a mock connection
type mockConn struct {
	readData  []byte
	writeData []byte
	readPos   int
}

func (m *mockConn) Read(p []byte) (n int, err error) {
	if m.readPos >= len(m.readData) {
		return 0, net.ErrClosed
	}

	n = copy(p, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockConn) Write(p []byte) (n int, err error) {
	m.writeData = append(m.writeData, p...)
	return len(p), nil
}

func (m *mockConn) Close() error {
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:1234")
	return addr
}

func (m *mockConn) RemoteAddr() net.Addr {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:5678")
	return addr
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestRC4Conn(t *testing.T) {
	mockConn := &mockConn{
		readData: []byte("test data"),
	}

	rc4Conn, err := NewRC4Conn(mockConn, "testkey")
	if err != nil {
		t.Fatalf("Failed to create RC4 connection: %v", err)
	}

	// Test write
	testData := []byte("hello world")
	n, err := rc4Conn.Write(testData)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if n != len(testData) {
		t.Errorf("Write returned wrong length: got %d, expected %d", n, len(testData))
	}

	// Test that data was encrypted (should be different)
	if string(mockConn.writeData) == string(testData) {
		t.Error("Data should be encrypted, but appears to be plaintext")
	}
}
