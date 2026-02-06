package store

import (
	"context"
	"sentinel-core/internal/domain/entity"

	"github.com/google/uuid"
	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	_, err := s.client.GetCollectionInfo(ctx, s.collectionName)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return s.client.CreateCollection(ctx, &qdrant.CreateCollection{
				CollectionName: s.collectionName,
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     dim,
					Distance: qdrant.Distance_Cosine,
				}),
			})
		}
		return err
	}
	return nil
}

func (s *QdrantStore) Search(ctx context.Context, vector []float32, threshold float32) (*entity.AIResponse, error) {
	// Query Qdrant for the closest match
	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collectionName,
		Query:          qdrant.NewQuery(vector...),
		Limit:          qdrant.PtrOf(uint64(1)),
		WithPayload:    qdrant.NewWithPayload(true),
		ScoreThreshold: &threshold,
	})
	if err != nil || len(res) == 0 {
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
