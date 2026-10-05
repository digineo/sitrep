package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/digineo/sitrep/internal/config"
)

// healthcheck asks the /healthz endpoint of a server running with the
// same configuration, for container health checks.
func healthcheck(args []string, stderr io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "healthcheck takes no arguments")
		return 2
	}

	dotenv, err := config.ReadDotenv(".env.local", ".env")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	env := config.NewEnv(config.Lookup(dotenv))
	listen := env.String("SITREP_LISTEN", ":2607")
	if err := env.Err(); err != nil {
		fmt.Fprintf(stderr, "invalid configuration:\n%v\n", err)
		return 1
	}

	url, err := healthURL(listen)
	if err != nil {
		fmt.Fprintf(stderr, "invalid configuration:\nSITREP_LISTEN: %v\n", err)
		return 1
	}

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stderr, "unhealthy: %s\n", resp.Status)
		return 1
	}
	return 0
}

// healthURL returns the /healthz URL for the listen address, on the
// loopback address if the server listens on all addresses.
func healthURL(listen string) (string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", err
	}

	if ip := net.ParseIP(host); host == "" || ip != nil && ip.IsUnspecified() {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/healthz", nil
}
