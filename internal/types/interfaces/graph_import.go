package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// GraphImportService imports caller-provided graph data into a knowledge base graph.
type GraphImportService interface {
	ImportGraph(ctx context.Context, kbID string, req *types.GraphImportRequest) (*types.GraphImportResult, error)
}
