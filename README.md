# Pivot Internal Tool

A SOCKS5 proxy tool with RC4 encryption for internal network pivoting.

## Overview

This tool consists of two components that support **multiple concurrent clients**:
- **Server**: Runs on the target internal network, accepts encrypted connections from multiple clients simultaneously
- **Client**: Runs on your machine(s), provides a local SOCKS5 proxy that encrypts traffic to the server

## Architecture

```
Client Machine 1                Internal Network
[Local SOCKS5]                  [Target Services]
192.168.1.1:1081 ----\                  ^
                      \                 |
Client Machine 2       RC4 Encrypted   |
[Local SOCKS5]        Connection    [Server]
192.168.1.2:1081 ----/             :1080
                     /                  |
Client Machine N    /                   |
[Local SOCKS5] ----/                    v
192.168.1.N:1081              [Internal Resources]

Multiple Clients              Single Server
```

## Usage

### Server Mode
Run this on the internal network machine:
```bash
./pivot-internal server -key secret -l :1080
```

Options:
- `-key`: Encryption key (must match client)
- `-l`: Listen address and port

### Client Mode
Run this on your local machine:
```bash
./pivot-internal client -key secret -r 10.10.10.10:1080 -l :1081
```

Options:
- `-key`: Encryption key (must match server)
- `-r`: Remote server address
- `-l`: Local SOCKS5 proxy listen address

### Using the Proxy
Once the client is running, configure your applications to use the SOCKS5 proxy at `127.0.0.1:1081`.

Examples:
```bash
# Using curl with SOCKS5 proxy
curl --socks5 127.0.0.1:1081 http://internal-server.local

# Using proxychains
echo "socks5 127.0.0.1 1081" >> /etc/proxychains.conf
proxychains nmap -sT internal-network.local

# Browser configuration
# Configure your browser to use SOCKS5 proxy: 127.0.0.1:1081
```

## Building

### Local Build
```bash
go build -o pivot-internal
```

### Cross-Platform Build
Use the provided build script for multiple platforms:
```bash
chmod +x build.sh
./build.sh
```

### Docker Build
Build Docker image locally:
```bash
# Build for current platform
docker build -t pivot-internal .

# Build for multiple platforms
docker buildx build --platform linux/amd64,linux/arm64 -t pivot-internal .
```

### GitHub Actions Automated Builds

This project includes comprehensive GitHub Actions workflows that automatically:

#### 1. **Build and Test** (Triggered on push/PR)
- Runs tests and builds binaries for all supported platforms
- Creates artifacts for Windows (amd64, 386), macOS (amd64, arm64), and Linux (amd64, arm64)
- Artifacts are available for download from the Actions tab

#### 2. **Docker Image Build** (Triggered on push to main, tags, and PRs)
- Automatically builds and pushes Docker images to GitHub Container Registry (GHCR)
- Multi-platform Docker images (linux/amd64, linux/arm64)
- Images are tagged based on branch/tag names
- Available at: `ghcr.io/hypnguyen1209/pivot-internal`

#### 3. **Release Creation** (Triggered on version tags)
- Creates GitHub releases with cross-platform binaries
- Includes SHA256 checksums for security verification
- Automatically generates release notes

#### 4. **Security Scanning** (Runs on push/PR and weekly)
- Vulnerability scanning with `govulncheck`
- Security analysis with `gosec`
- Dependency checking with `nancy`

### Using Pre-built Docker Images

Pull the latest Docker image:
```bash
docker pull ghcr.io/hypnguyen1209/pivot-internal:latest
```

Run server in Docker:
```bash
docker run -p 1080:1080 ghcr.io/hypnguyen1209/pivot-internal:latest ./pivot-internal server -key secret -l :1080
```

Run client in Docker:
```bash
docker run -p 1081:1081 ghcr.io/hypnguyen1209/pivot-internal:latest ./pivot-internal client -key secret -r server-ip:1080 -l :1081
```

### Creating a Release

To trigger an automated release with binaries:
```bash
git tag v1.0.0
git push origin v1.0.0
```

This will automatically:
- Build binaries for all supported platforms
- Create a GitHub release with download links
- Generate checksums for security verification
- Push Docker images with version tags

## Features

- ✅ SOCKS5 proxy protocol support
- ✅ RC4 encryption for traffic obfuscation
- ✅ IPv4, IPv6, and domain name resolution
- ✅ **Multiple concurrent clients** support
- ✅ **Connection tracking and logging** with unique IDs
- ✅ **Graceful shutdown** with signal handling (Ctrl+C)
- ✅ Cross-platform support (Windows, Linux, macOS)
- ✅ **Concurrent connection handling** per client

## Security Notes

- RC4 is used for encryption. While not the strongest encryption, it provides good obfuscation for network traffic
- Ensure your encryption key is strong and kept secret
- This tool is designed for authorized penetration testing and internal network assessment only

## Example Workflow

### Single Server, Multiple Clients Setup

1. **Deploy server on internal network (10.10.10.10)**:
   ```bash
   ./pivot-internal server -key MySecretKey123 -l :1080
   ```

2. **Start clients from different machines**:

   **Machine A (192.168.1.100)**:
   ```bash
   ./pivot-internal client -key MySecretKey123 -r 10.10.10.10:1080 -l :1081
   ```

   **Machine B (192.168.1.101)**:
   ```bash
   ./pivot-internal client -key MySecretKey123 -r 10.10.10.10:1080 -l :1081
   ```

   **Machine C (192.168.1.102)**:
   ```bash
   ./pivot-internal client -key MySecretKey123 -r 10.10.10.10:1080 -l :1081
   ```

3. **Each machine can now use their local proxy**:

   **From Machine A**:
   ```bash
   curl --socks5 192.168.1.100:1081 http://internal-server.local
   ```

   **From Machine B**:
   ```bash
   curl --socks5 192.168.1.101:1081 http://internal-server.local
   ```

   **From Machine C**:
   ```bash
   curl --socks5 192.168.1.102:1081 http://internal-server.local
   ```

### Team Collaboration Scenario

This setup allows multiple team members to simultaneously access internal resources through a single pivot point:

- **Pentester 1** (192.168.1.100): Running vulnerability scans
- **Pentester 2** (192.168.1.101): Manual web application testing  
- **Pentester 3** (192.168.1.102): Network reconnaissance

All traffic is encrypted and routed through the same internal server, providing a coordinated attack surface while maintaining operational security.
