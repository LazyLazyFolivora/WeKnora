package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"encoding/json"
	filesvc "github.com/Tencent/WeKnora/internal/application/service/file"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

type cleanupTenantRepo struct{ interfaces.TenantRepository }

func (cleanupTenantRepo) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id}, nil
}

type cleanupCatalog struct {
	fakeCatalog
	records map[string]*types.StoredResource
}

func (c *cleanupCatalog) Resolve(_ context.Context, ref string) (*types.StoredResource, error) {
	r := c.records[ref]
	if r == nil || r.State == types.ResourceStateDeleted {
		return nil, errors.New("resource not found")
	}
	return r, nil
}

// The deletion-only lookup must keep tombstones usable by a partial-task retry.
func (c *cleanupCatalog) ResolveForDeletion(_ context.Context, ref string) (*types.StoredResource, error) {
	r := c.records[ref]
	if r == nil {
		return nil, errors.New("resource not found")
	}
	return r, nil
}
func (c *cleanupCatalog) ResolvePath(ctx context.Context, ref string) (string, *types.StoredResource, error) {
	if _, ok := types.ParseResourcePath(ref); !ok {
		return ref, nil, nil
	}
	r, e := c.Resolve(ctx, ref)
	if e != nil {
		return "", nil, e
	}
	return r.PhysicalPath, r, nil
}
func (c *cleanupCatalog) MarkDeleted(_ context.Context, ref string) error {
	c.records[ref].State = types.ResourceStateDeleted
	return nil
}
func (c *cleanupCatalog) Release(context.Context, string, string, string) (int64, error) {
	return 0, nil
}

type cleanupResolver struct {
	interfaces.StorageBackendResolver
	svc     interfaces.FileService
	backend string
	tenant  uint64
	calls   int
}

func (r *cleanupResolver) ResolveFileService(_ context.Context, t *types.Tenant, b, p, d string) (interfaces.FileService, string, error) {
	r.calls++
	if b != r.backend || t.ID != r.tenant {
		return nil, "", errors.New("wrong backend or tenant")
	}
	return r.svc, "local", nil
}

type cleanupFailFiles struct {
	fakeFileService
	err error
}

func (f *cleanupFailFiles) DeleteFile(context.Context, string) error { return f.err }

func cleanupTask(t *testing.T) *asynq.Task {
	t.Helper()
	b, e := json.Marshal(types.KBDeletePayload{TenantID: 7, KnowledgeBaseID: "kb"})
	require.NoError(t, e)
	return asynq.NewTask(types.TypeKBDelete, b)
}
func cleanupService(repo *kbDeleteTrackingKnowledgeRepo) *knowledgeBaseService {
	return &knowledgeBaseService{kgRepo: repo, chunkRepo: kbCleanupChunkRepo{}, modelService: kbCleanupModelService{}, tenantRepo: cleanupTenantRepo{}}
}
func TestProcessKBDeleteResolvesRegisteredBackend(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "7", "knowledge", "file.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, []byte("independent cleanup fixture"), 0600))
	ref := handleRef("a")
	backend := "00000000-0000-0000-0000-000000000007"
	catalog := &cleanupCatalog{records: map[string]*types.StoredResource{ref: {TenantID: 7, StorageBackendID: backend, Provider: "local", PhysicalPath: types.BuildStorageBackendPath(backend, "local://7/knowledge/file.md"), State: types.ResourceStateActive}}}
	repo := &kbDeleteTrackingKnowledgeRepo{populatedKBKnowledgeRepo: populatedKBKnowledgeRepo{items: []*types.Knowledge{{ID: "knowledge", KnowledgeBaseID: "kb", FilePath: ref, EmbeddingModelID: "model"}}}}
	svc := cleanupService(repo)
	svc.resourceCatalog = catalog
	svc.fileSvc = filesvc.NewResourceCatalogFileService(filesvc.NewLocalFileService(t.TempDir(), ""), catalog)
	resolver := &cleanupResolver{backend: backend, tenant: 7, svc: filesvc.NewResourceCatalogFileService(filesvc.NewBackendScopedFileService(backend, filesvc.NewLocalFileService(root, "")), catalog)}
	svc.storageResolver = resolver
	require.NoError(t, svc.ProcessKBDelete(context.Background(), cleanupTask(t)))
	_, err := os.Stat(path)
	require.True(t, errors.Is(err, os.ErrNotExist), "native cleanup must remove the registered backend's real file")
	require.Greater(t, resolver.calls, 0)
	require.Equal(t, 1, repo.deleteCalls)
}
func TestProcessKBDeletePhysicalFailureRemainsRetryable(t *testing.T) {
	repo := &kbDeleteTrackingKnowledgeRepo{populatedKBKnowledgeRepo: populatedKBKnowledgeRepo{items: []*types.Knowledge{{ID: "knowledge", KnowledgeBaseID: "kb", FilePath: "local://7/knowledge/file.md", EmbeddingModelID: "model"}}}}
	svc := cleanupService(repo)
	failure := errors.New("owned storage unavailable")
	files := &cleanupFailFiles{err: failure}
	svc.fileSvc = files
	require.ErrorIs(t, svc.ProcessKBDelete(context.Background(), cleanupTask(t)), failure)
	require.Zero(t, repo.deleteCalls, "failed physical cleanup must retain knowledge rows for the async retry")
	files.err = nil
	require.NoError(t, svc.ProcessKBDelete(context.Background(), cleanupTask(t)))
	require.Equal(t, 1, repo.deleteCalls)
}

type cleanupSelectiveFailure struct {
	interfaces.FileService
	failPath string
	err      error
}

func (f *cleanupSelectiveFailure) DeleteFile(ctx context.Context, path string) error {
	if path == f.failPath && f.err != nil {
		return f.err
	}
	return f.FileService.DeleteFile(ctx, path)
}
func TestProcessKBDeletePartialFilesRetryWithoutRevivingDeletedResource(t *testing.T) {
	root := t.TempDir()
	backend := "00000000-0000-0000-0000-000000000007"
	catalog, _ := newResourceCatalogForTest(t)
	items := []*types.Knowledge{}
	paths := []string{}
	for _, id := range []string{"first", "second"} {
		path := filepath.Join(root, "7", id, "file.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte("cleanup retry fixture"), 0600))
		paths = append(paths, path)
		ref, err := catalog.Register(context.Background(), 7, types.BuildStorageBackendPath(backend, "local://7/"+id+"/file.md"), interfaces.ResourceRegistration{})
		require.NoError(t, err)
		require.NoError(t, catalog.Bind(context.Background(), ref, types.ResourceOwnerKnowledge, id, types.ResourceRelationSourceFile))
		items = append(items, &types.Knowledge{ID: id, KnowledgeBaseID: "kb", FilePath: ref, EmbeddingModelID: "model"})
	}
	repo := &kbDeleteTrackingKnowledgeRepo{populatedKBKnowledgeRepo: populatedKBKnowledgeRepo{items: items}}
	svc := cleanupService(repo)
	svc.resourceCatalog = catalog
	failure := errors.New("second object temporarily unavailable")
	disk := &cleanupSelectiveFailure{FileService: filesvc.NewBackendScopedFileService(backend, filesvc.NewLocalFileService(root, "")), failPath: types.BuildStorageBackendPath(backend, "local://7/second/file.md"), err: failure}
	svc.storageResolver = &cleanupResolver{backend: backend, tenant: 7, svc: filesvc.NewResourceCatalogFileService(disk, catalog)}
	require.ErrorIs(t, svc.ProcessKBDelete(context.Background(), cleanupTask(t)), failure)
	require.Zero(t, repo.deleteCalls)
	_, err := catalog.Resolve(context.Background(), items[0].FilePath)
	require.Error(t, err, "deleted resource must remain unreadable")
	disk.err = nil
	require.NoError(t, svc.ProcessKBDelete(context.Background(), cleanupTask(t)))
	require.Equal(t, 1, repo.deleteCalls)
	for _, path := range paths {
		_, err := os.Stat(path)
		require.True(t, errors.Is(err, os.ErrNotExist))
	}
}
