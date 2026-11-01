package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/models/embedding"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type embeddingInitKnowledgeRepo struct {
	interfaces.KnowledgeRepository
	stored types.Knowledge
}

func (r *embeddingInitKnowledgeRepo) GetKnowledgeByID(context.Context, uint64, string) (*types.Knowledge, error) {
	copy := r.stored
	return &copy, nil
}

func (r *embeddingInitKnowledgeRepo) UpdateKnowledge(_ context.Context, knowledge *types.Knowledge) error {
	r.stored = *knowledge
	return nil
}

type embeddingInitKBService struct {
	interfaces.KnowledgeBaseService
	kb *types.KnowledgeBase
}

func (s embeddingInitKBService) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

type embeddingInitTenantRepo struct {
	interfaces.TenantRepository
}

func (embeddingInitTenantRepo) GetTenantByID(context.Context, uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: 1}, nil
}

type embeddingInitTenantService struct{ interfaces.TenantService }

func (embeddingInitTenantService) GetWeKnoraCloudCredentials(context.Context) *types.WeKnoraCloudCredentials {
	return nil
}

type embeddingInitModelService struct {
	interfaces.ModelService
	err   error
	calls int
}

func (s *embeddingInitModelService) GetEmbeddingModel(context.Context, string) (embedding.Embedder, error) {
	s.calls++
	return nil, s.err
}

type embeddingInitFileService struct{ interfaces.FileService }

func (embeddingInitFileService) GetFile(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("Local document fixture.")), nil
}

// Exercise the task entry points and the real span persistence/cascade. A model
// initialization error must never become an acknowledged success with a running
// root, regardless of whether text came from a file, passages, or a manual edit.
func TestEmbeddingInitFailureTerminatesKnowledgeTask(t *testing.T) {
	for _, entry := range []string{"file", "passages", "manual"} {
		t.Run(entry, func(t *testing.T) {
			ctx := context.Background()
			tracker, db := setupSpanTrackerTest(t)
			knowledge := types.Knowledge{
				ID: "knowledge-init-failure", TenantID: 1, KnowledgeBaseID: "kb-1",
				ParseStatus: types.ParseStatusPending, FileName: "fixture.md", FileType: "md",
			}
			repo := &embeddingInitKnowledgeRepo{stored: knowledge}
			cause := errors.New("base URL SSRF check failed: direct IP address access is not allowed")
			models := &embeddingInitModelService{err: cause}
			svc := &knowledgeService{
				repo: repo,
				kbService: embeddingInitKBService{kb: &types.KnowledgeBase{
					ID: "kb-1", TenantID: 1, EmbeddingModelID: "embedding-1",
					IndexingStrategy: types.IndexingStrategy{VectorEnabled: true},
				}},
				tenantRepo: embeddingInitTenantRepo{}, tenantService: embeddingInitTenantService{}, modelService: models,
				spanTracker: tracker, fileSvc: embeddingInitFileService{},
				// Chunk/index/enqueue dependencies intentionally absent: none
				// may be reached after a failed embedder initialization.
			}
			var taskErr error
			if entry == "manual" {
				payload, err := json.Marshal(types.ManualProcessPayload{
					TenantID: 1, KnowledgeID: knowledge.ID, KnowledgeBaseID: "kb-1",
					Content: "Local manual fixture.",
				})
				require.NoError(t, err)
				taskErr = svc.ProcessManualUpdate(ctx, asynq.NewTask(types.TypeManualProcess, payload))
			} else {
				payload := types.DocumentProcessPayload{
					TenantID: 1, KnowledgeID: knowledge.ID, KnowledgeBaseID: "kb-1",
					FileName: "fixture.md", FileType: "md",
				}
				if entry == "file" {
					payload.FilePath = "local://fixture.md"
				} else {
					payload.Passages = []string{"Local passage fixture."}
				}
				raw, err := json.Marshal(payload)
				require.NoError(t, err)
				taskErr = svc.ProcessDocument(ctx, asynq.NewTask(types.TypeDocumentProcess, raw))
			}
			assert.ErrorIs(t, taskErr, asynq.SkipRetry, "archive the failed task without replaying prior stages")
			assert.ErrorIs(t, taskErr, cause, "retain the actionable initialization error")
			assert.Equal(t, 1, models.calls)
			assert.Equal(t, types.ParseStatusFailed, repo.stored.ParseStatus)
			assert.Contains(t, repo.stored.ErrorMessage, cause.Error())
			assert.Zero(t, repo.stored.PendingSubtasksCount)

			var root types.KnowledgeProcessingSpan
			require.NoError(t, db.Where("knowledge_id = ? AND attempt = 1 AND kind = ?", knowledge.ID, types.SpanKindRoot).First(&root).Error)
			assert.Equal(t, types.SpanStatusFailed, root.Status)
			assert.NotNil(t, root.FinishedAt)
			var failedStage types.KnowledgeProcessingSpan
			if assert.NoError(t, db.Where("knowledge_id = ? AND attempt = 1 AND name = ?", knowledge.ID, types.StageEmbedding).First(&failedStage).Error) {
				assert.Equal(t, types.SpanStatusFailed, failedStage.Status)
				assert.NotEmpty(t, failedStage.ErrorCode)
				assert.Contains(t, failedStage.ErrorDetail, cause.Error())
			}
		})
	}
}
