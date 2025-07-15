package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  ./pivot-internal server -key <secret> -l <listen_addr>")
		fmt.Println("  ./pivot-internal server -key <secret> -c agent  (starts in agent mode)")
		fmt.Println("  ./pivot-internal agent -key <secret> -l <listen_addr> -i <internal_addr>")
		fmt.Println("  ./pivot-internal client -key <secret> -r <remote_addr> -l <local_addr>")
		os.Exit(1)
	}

	mode := os.Args[1]

	switch mode {
	case "server":
		runServer()
	case "agent":
		runAgent()
	case "client":
		runClient()
	default:
		fmt.Printf("Unknown mode: %s\n", mode)
		os.Exit(1)
	}
}

func runServer() {
	serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
	key := serverCmd.String("key", "", "Encryption key")
	listen := serverCmd.String("l", ":1080", "Listen address")
	connect := serverCmd.String("c", "", "Agent server address to connect to")

	serverCmd.Parse(os.Args[2:])

	if *key == "" {
		log.Fatal("Key is required")
	}

	var server *Server
	if *connect != "" {
		// Agent mode - server connects to agent
		fmt.Printf("Starting server connecting to agent at %s with key: %s\n", *connect, *key)
		server = NewServer(*key, *connect)
	} else {
		// Traditional listen mode
		if *listen == "" {
			*listen = ":1080"
		}
		fmt.Printf("Starting server on %s with key: %s\n", *listen, *key)
		server = NewServer(*key, *listen)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()

		// Give server time to cleanup
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		server.Shutdown(shutdownCtx)
		log.Println("Server shutdown complete")

	case err := <-errChan:
		if err != nil {
			log.Fatal("Server error:", err)
		}
	}
}

func runAgent() {
	agentCmd := flag.NewFlagSet("agent", flag.ExitOnError)
	key := agentCmd.String("key", "", "Encryption key")
	listen := agentCmd.String("l", ":1080", "Listen address for clients")
	internal := agentCmd.String("i", ":8000", "Internal listen address for victim server")

	agentCmd.Parse(os.Args[2:])

	if *key == "" {
		log.Fatal("Key is required")
	}

	fmt.Printf("Starting agent server: client listen=%s, internal listen=%s with key: %s\n", *listen, *internal, *key)

	agent := NewAgent(*key, *listen, *internal)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start agent in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- agent.Start(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()

		// Give agent time to cleanup
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		agent.Shutdown(shutdownCtx)
		log.Println("Agent shutdown complete")

	case err := <-errChan:
		if err != nil {
			log.Fatal("Agent error:", err)
		}
	}
}

func runClient() {
	clientCmd := flag.NewFlagSet("client", flag.ExitOnError)
	key := clientCmd.String("key", "", "Encryption key")
	remote := clientCmd.String("r", "", "Remote server address")
	local := clientCmd.String("l", ":1081", "Local listen address")

	clientCmd.Parse(os.Args[2:])

	if *key == "" || *remote == "" {
		log.Fatal("Key and remote address are required")
	}

	fmt.Printf("Starting client: local=%s -> remote=%s with key: %s\n", *local, *remote, *key)

	client := NewClient(*key, *remote, *local)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start client in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- client.Start(ctx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()

		// Give client time to cleanup
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		client.Shutdown(shutdownCtx)
		log.Println("Client shutdown complete")

	case err := <-errChan:
		if err != nil {
			log.Fatal("Client error:", err)
		}
	}
}
