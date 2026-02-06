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

func (s *QdrantStore) Search(ctx context.Context, vector []float32, threshold float32, filters map[string]string) (*entity.AIResponse, float32, string, error) {
    // 1. Construct the Filter
    var qdrantFilter *qdrant.Filter
    if len(filters) > 0 {
        var mustConditions []*qdrant.Condition
        for key, value := range filters {
            // NewMatch handles keyword/string equality
            mustConditions = append(mustConditions, qdrant.NewMatch(key, value))
        }
        qdrantFilter = &qdrant.Filter{
            Must: mustConditions,
        }
    }

    // 2. Execute the Query
    res, err := s.client.Query(ctx, &qdrant.QueryPoints{
        CollectionName: s.collectionName,
        Query:          qdrant.NewQuery(vector...),
        Filter:         qdrantFilter, // Pass the filter here
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

func (s *QdrantStore) Save(ctx context.Context, prompt string, resp *entity.AIResponse, vector []float32, metadata map[string]any) error {
    // Prepare base payload
    payload := map[string]any{
        "prompt":  prompt,
        "content": resp.Content,
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