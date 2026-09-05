package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// RetrieveGraphRepository is a repository for retrieving graphs
type RetrieveGraphRepository interface {
	// AddGraph adds a graph to the repository
	AddGraph(ctx context.Context, namespace types.NameSpace, graphs []*types.GraphData) error
	// DelGraph deletes a graph from the repository
	DelGraph(ctx context.Context, namespace []types.NameSpace) error
	// SearchNode searches for nodes in the repository
	SearchNode(ctx context.Context, namespace types.NameSpace, nodes []string) (*types.GraphData, error)
	// SearchByCypher executes a read-only Cypher query and returns parsed graph data.
	// The Cypher must RETURN n, r, m (source node, relationship, target node).
	SearchByCypher(ctx context.Context, cypher string, params map[string]interface{}) (*types.GraphData, error)
}
