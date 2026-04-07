package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Protocol selects the transport layer for the Qdrant client.
type Protocol string

const (
	ProtocolHTTP Protocol = "http"
	ProtocolGRPC Protocol = "grpc"
)


// Config holds all Qdrant connection and collection parameters.
type Config struct {
	// Endpoint is the full Qdrant address including port, e.g.
	// "http://localhost:6333" (HTTP) or "localhost:6334" (gRPC).
	Endpoint string

	Collection string

	// Dimension is the vector size; must match the embedding model output.
	Dimension int

	// Protocol selects HTTP (REST) or gRPC transport.
	// Defaults to ProtocolHTTP if empty.
	Protocol Protocol

	// APIKey is optional — required when connecting to Qdrant Cloud.
	APIKey string
}

// Point is the canonical in-memory representation of one vector record.
// It is transport-agnostic; each backend converts it to the wire format.
type Point struct {
	// ID must be a UUID string or a uint64 string. Qdrant rejects other formats.
	ID      string
	Vector  []float32
	Payload map[string]any
}

// Client is the backend-agnostic interface every transport must satisfy.
// All methods accept a context so callers can enforce timeouts.
type Client interface {
	// EnsureCollection creates the collection if it does not exist.
	// Safe to call on every startup (idempotent).
	EnsureCollection(ctx context.Context, vectorSize int) error

	// UpsertPoints writes points using an upsert semantic
	// (insert or overwrite by ID).
	UpsertPoints(ctx context.Context, points []Point) error

	// Close releases transport resources (connections, gRPC channels).
	Close() error
}

// New constructs the correct Client implementation based on cfg.Protocol.
func New(cfg Config) (Client, error) {
	proto := cfg.Protocol
	if proto == "" {
		proto = ProtocolHTTP
	}

	switch proto {
	case ProtocolHTTP:
		return newHTTPClient(cfg)
	case ProtocolGRPC:
		return newGRPCClient(cfg)
	default:
		return nil, fmt.Errorf("qdrant: unknown protocol %q (use \"http\" or \"grpc\")", proto)
	}
}

// ParseEndpoint splits a host+port string into its components.
// The scheme defaults to "http" when absent.
func ParseEndpoint(input string) (scheme, host string, port int, err error) {
	if !strings.Contains(input, "://") {
		input = "http://" + input
	}

	u, err := url.Parse(input)
	if err != nil {
		return "", "", 0, fmt.Errorf("qdrant: invalid endpoint %q: %w", input, err)
	}

	h, p, splitErr := net.SplitHostPort(u.Host)
	if splitErr != nil {
		// No port in the URL — return host only; caller picks a default.
		return u.Scheme, u.Hostname(), 0, nil
	}

	portInt, err := strconv.Atoi(p)
	if err != nil {
		return "", "", 0, fmt.Errorf("qdrant: invalid port in %q: %w", input, err)
	}

	return u.Scheme, h, portInt, nil
}
