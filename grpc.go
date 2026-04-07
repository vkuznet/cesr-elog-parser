package main

// gRPC transport for Qdrant.
//
// Qdrant exposes two gRPC services on port 6334 (default):
//   - qdrant.Collections  — create / delete / info
//   - qdrant.Points       — upsert / search / delete
//
// The generated protobuf stubs live in the official SDK:
//   github.com/qdrant/go-client
//
// go.mod entry:
//   require github.com/qdrant/go-client v1.8.0
//
// The import paths used below map to that module:
//   github.com/qdrant/go-client/qdrant         — client constructors
//
// If you are vendoring or generating your own stubs, adjust the imports
// and the grpcPoint helper at the bottom accordingly.

import (
	"context"
	"fmt"
	"strings"

	qdrantSDK "github.com/qdrant/go-client/qdrant"
)

const defaultGRPCPort = 6334

// grpcClient implements Client over Qdrant's gRPC API.
type grpcClient struct {
	collection string
	client     *qdrantSDK.Client // official SDK wrapper
}

func newGRPCClient(cfg Config) (*grpcClient, error) {
	_, host, port, err := ParseEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if port == 0 {
		port = defaultGRPCPort
	}

	sdkCfg := &qdrantSDK.Config{
		Host: host,
		Port: port,
	}
	if cfg.APIKey != "" {
		sdkCfg.APIKey = cfg.APIKey
		sdkCfg.UseTLS = true // Qdrant Cloud requires TLS with an API key
	}

	c, err := qdrantSDK.NewClient(sdkCfg)
	if err != nil {
		return nil, fmt.Errorf("qdrant grpc: dial %s:%d: %w", host, port, err)
	}

	return &grpcClient{
		collection: cfg.Collection,
		client:     c,
	}, nil
}

// ── Public interface methods ─────────────────────────────────────────────────

func (g *grpcClient) EnsureCollection(ctx context.Context, vectorSize int) error {
	// GetCollection returns an error when the collection does not exist.
	colClient := g.client.GetCollectionsClient()
	_, err := colClient.Get(ctx, &qdrantSDK.GetCollectionInfoRequest{
		CollectionName: g.collection,
	})
	if err == nil {
		return nil // already exists
	}

	// Distinguish "not found" from a real transport error.
	// The SDK wraps gRPC status codes; check via status.Code if needed.
	// For simplicity we attempt creation and treat "already exists" as success.
	err = g.client.CreateCollection(ctx, &qdrantSDK.CreateCollection{
		CollectionName: g.collection,
		VectorsConfig: qdrantSDK.NewVectorsConfig(&qdrantSDK.VectorParams{
			Size:     uint64(vectorSize),
			Distance: qdrantSDK.Distance_Cosine,
		}),
	})
	if err != nil {
		// Qdrant returns ALREADY_EXISTS when the collection was created concurrently.
		// Treat that as success so EnsureCollection is idempotent.
		if isAlreadyExistsErr(err) {
			return nil
		}
		return fmt.Errorf("qdrant grpc: create collection: %w", err)
	}
	return nil
}

func (g *grpcClient) UpsertPoints(ctx context.Context, points []Point) error {
	wire := make([]*qdrantSDK.PointStruct, len(points))
	for i, p := range points {
		wire[i] = toSDKPoint(p)
	}

	waitTrue := true
	_, err := g.client.Upsert(ctx, &qdrantSDK.UpsertPoints{
		CollectionName: g.collection,
		Wait:           &waitTrue,
		Points:         wire,
	})
	if err != nil {
		return fmt.Errorf("qdrant grpc: upsert: %w", err)
	}
	return nil
}

func (g *grpcClient) Close() error {
	return g.client.Close()
}

// ── Conversion helpers ───────────────────────────────────────────────────────

// toSDKPoint converts our transport-agnostic Point into the SDK's PointStruct.
func toSDKPoint(p Point) *qdrantSDK.PointStruct {
	var err error
	var val *qdrantSDK.Value
	payload := make(map[string]*qdrantSDK.Value, len(p.Payload))
	for k, v := range p.Payload {
		val, err = toSDKValue(v)
		if err == nil {
			payload[k] = val
		}
	}

	return &qdrantSDK.PointStruct{
		Id:      qdrantSDK.NewID(p.ID),
		Vectors: qdrantSDK.NewVectors(p.Vector...),
		Payload: payload,
	}
}

// toSDKValue converts a Go any value to qdrant.Value.
// Handles the types most likely to appear in elog payloads.
func toSDKValue(v any) (*qdrantSDK.Value, error) {
	switch val := v.(type) {
	case string:
		return qdrantSDK.NewValue(val)
	case bool:
		return qdrantSDK.NewValue(val)
	case int:
		return qdrantSDK.NewValue(int64(val))
	case int64:
		return qdrantSDK.NewValue(val)
	case float32:
		return qdrantSDK.NewValue(float64(val))
	case float64:
		return qdrantSDK.NewValue(val)
	case nil:
		return &qdrantSDK.Value{Kind: &qdrantSDK.Value_NullValue{}}, nil
	default:
		// Fallback: stringify unknown types rather than panic.
		return qdrantSDK.NewValue(fmt.Sprintf("%v", val))
	}
}

// isAlreadyExistsErr checks whether a gRPC error has the ALREADY_EXISTS code.
func isAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	// google.golang.org/grpc/status
	// import "google.golang.org/grpc/codes"
	// s, ok := status.FromError(err)
	// return ok && s.Code() == codes.AlreadyExists
	//
	// Inline string check as a fallback if grpc/status is not yet in go.mod:
	sval := fmt.Sprintf("%v", err)
	return err != nil && strings.Contains(sval, "AlreadyExists")
}
