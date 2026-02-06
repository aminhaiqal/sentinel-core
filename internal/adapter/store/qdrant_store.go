package store

import (
	"context"
	"fmt"
	"log"
	"sentinel-core/internal/domain/entity"
	"time"

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
			// 1. Create the Collection
			err := s.client.CreateCollection(ctx, &qdrant.CreateCollection{
				CollectionName: s.collectionName,
				VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
					Size:     dim,
					Distance: qdrant.Distance_Cosine,
				}),
			})
			if err != nil {
				return fmt.Errorf("failed to create collection: %w", err)
			}
		} else {
			return err
		}
	}

	// 2. Create the Payload Index for the Freshness Filter (TTL)
	// This makes range queries on "created_at" lightning fast.
	// We use the 'Wait' flag to ensure the index is ready before we start.
	_, err = s.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
		CollectionName: s.collectionName,
		FieldName:      "created_at",
		FieldType:      qdrant.FieldType_FieldTypeInteger.Enum(),
		Wait:           qdrant.PtrOf(true),
	})

	if err != nil {
		// Log but don't fail if index already exists
		log.Printf("[QDRANT] Warning: Could not create created_at index (might already exist): %v", err)
	}

	return nil
}

func (s *QdrantStore) Search(ctx context.Context, vector []float32, threshold float32, filters map[string]string) (*entity.AIResponse, float32, string, error) { // 1. Construct the Filter
	var mustConditions []*qdrant.Condition

	// 1. Add Existing Metadata Filters (User ID, Source, etc.)
	for key, value := range filters {
		mustConditions = append(mustConditions, qdrant.NewMatch(key, value))
	}

	// 2. Add Freshness Filter (The TTL)
	// Only return results created in the last 24 hours
	oneDayAgo := time.Now().Add(-24 * time.Hour).Unix()
	mustConditions = append(mustConditions, &qdrant.Condition{
		ConditionOneOf: &qdrant.Condition_Field{
			Field: &qdrant.FieldCondition{
				Key: "created_at",
				Range: &qdrant.Range{
					Gte: qdrant.PtrOf(float64(oneDayAgo)), // Greater than or equal to
				},
			},
		},
	})

	res, err := s.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: s.collectionName,
		Query:          qdrant.NewQuery(vector...),
		Filter:         &qdrant.Filter{Must: mustConditions},
		Limit:          qdrant.PtrOf(uint64(1)),
		WithPayload:    qdrant.NewWithPayload(true),
		ScoreThreshold: &threshold,
	})

	if err != nil || len(res) == 0 {
		return nil, 0, "", err
	}

	hit := res[0]
	payload := hit.Payload

	// 3. Extract data
	originalPrompt := payload["prompt"].GetStringValue()
	content := payload["content"].GetStringValue()

	response := &entity.AIResponse{
		Content: content,
		Cached:  true,
	}

	return response, hit.Score, originalPrompt, nil
}

func (s *QdrantStore) Save(ctx context.Context, prompt string, resp *entity.AIResponse, vector []float32, metadata map[string]any) error { // Prepare base payload
	payload := map[string]any{
		"prompt":     prompt,
		"content":    resp.Content,
		"created_at": time.Now().Unix(), // Store as Unix integer
	}

	// Merge in extra metadata (e.g., user_id, source_account)
	for k, v := range metadata {
		payload[k] = v
	}

	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collectionName,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewIDUUID(uuid.NewString()),
				Vectors: qdrant.NewVectors(vector...),
				Payload: qdrant.NewValueMap(payload),
			},
		},
	})
	return err
}
