package store

import (
	"context"
	"sentinel-core/internal/domain/entity"
	"strings"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
)

type QdrantStore struct {
	client         *qdrant.Client
	collectionName string
}

func NewQdrantStore(client *qdrant.Client, collectionName string) *QdrantStore {
	return &QdrantStore{
		client:         client,
		collectionName: collectionName,
	}
}

func (s *QdrantStore) InitCollection(ctx context.Context, dim uint64) error {
	// Use GetCollectionInfo to check if it exists
	_, err := s.client.GetCollectionInfo(ctx, s.collectionName)

	if err != nil {
		// In Qdrant's Go SDK, if a collection is missing, it usually returns a gRPC error
		// containing "Not Found". If the error is nil, the collection exists.
		if strings.Contains(err.Error(), "Not Found") || strings.Contains(err.Error(), "404") {
			return s.client.CreateCollection(ctx, &qdrant.CreateCollection{
				CollectionName: s.collectionName,
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     dim,
					Distance: qdrant.Distance_Cosine,
				}),
			})
		}
		return err // Some other connection error
	}

	return nil // Collection already exists
}

func (s *QdrantStore) Search(ctx context.Context, vector []float32) (*entity.AIResponse, error) {
	// Query Qdrant for the closest match
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collectionName,
		Query:          qdrant.NewQuery(vector...),
		Limit:          qdrant.PtrOf(uint64(1)),
		WithPayload:    qdrant.NewWithPayload(true),
	})
	if err != nil || len(res) == 0 || res[0].Score < 0.90 { // Threshold for "semantic hit"
		return nil, nil
	}

	// Map payload back to AIResponse
	return &entity.AIResponse{
		Content: res[0].Payload["content"].GetStringValue(),
		Cached:  true,
	}, nil
}

func (s *QdrantStore) Save(ctx context.Context, prompt string, resp *entity.AIResponse, vector []float32) error {
	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collectionName,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewIDUUID(uuid.NewString()),
				Vectors: qdrant.NewVectors(vector...),
				Payload: qdrant.NewValueMap(map[string]any{
					"prompt":  prompt,
					"content": resp.Content,
				}),
			},
		},
	})
	return err
}
