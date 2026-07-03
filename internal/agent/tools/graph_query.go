package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
)

var graphQueryTool = BaseTool{
	name: ToolGraphQuery,
	description: `Execute any read-only Cypher query against the Neo4j knowledge graph.

## Core Function
Runs whatever valid Cypher query you provide and returns the matched nodes and relationships. This is a general-purpose graph query tool — you are responsible for writing the right query.

## Schema Discovery
When you are unfamiliar with the graph structure, start by exploring:
- CALL db.labels() — discover all node labels in the graph
- CALL db.relationshipTypes() — discover all relation types
- MATCH (n:SomeLabel) RETURN n LIMIT 3 — inspect sample nodes and their properties
- MATCH (n)-[r]->(m) RETURN n, r, m LIMIT 10 — sample the graph structure

Once you understand the schema, write targeted MATCH queries.

## Graph Structure Overview
The graph contains two categories of nodes:
- **GraphEntity** nodes: your knowledge graph entities with properties name, entity_name, entity_type, entity_data, source_site, confidence_score
- **External nodes** (drug, disease, gene/protein, pathway, etc.): PrimeKG reference data with properties name, primekg_id, primekg_type, node_source
- Bridge: (GraphEntity)-[:REFERENCES]->(external node) connects your entities to PrimeKG

## Query Guidelines
- **CRITICAL: Always RETURN node/relationship objects (n, r, m), NEVER scalar properties (n.name, n.entity_type). The parser only handles Node and Relationship objects, not strings.**
  - CORRECT: `RETURN n` / `RETURN n, r, m`
  - WRONG: `RETURN n.name, n.entity_type, n.entity_data`
- Use MATCH for data queries, CALL for schema introspection
- Prefer CONTAINS for fuzzy name matching: WHERE toLower(n.name) CONTAINS toLower("keyword")
- Always add LIMIT (recommend 50-100) to avoid overwhelming results
- You may use node labels (e.g., :GraphEntity, :drug, :disease) to narrow scope
- For multi-hop traversal, chain MATCH patterns: (a)-[:TREATS]->(b)-[:REFERENCES]->(c)

## Safety
All queries are validated before execution. Only MATCH and CALL statements are allowed. DELETE, SET, CREATE, MERGE, DROP, REMOVE are forbidden.`,
	schema: utils.GenerateSchema[GraphQueryInput](),
}

// GraphQueryInput defines the input for graph_query.
type GraphQueryInput struct {
	Cypher string `json:"cypher" jsonschema:"required, Read-only Cypher query (MATCH or CALL)"`
}

// GraphQueryTool executes arbitrary read-only Cypher queries against Neo4j.
type GraphQueryTool struct {
	BaseTool
	graphRepo interfaces.RetrieveGraphRepository
}

// NewGraphQueryTool creates a new graph_query tool.
func NewGraphQueryTool(graphRepo interfaces.RetrieveGraphRepository) *GraphQueryTool {
	return &GraphQueryTool{
		BaseTool:  graphQueryTool,
		graphRepo: graphRepo,
	}
}

// Execute runs the Cypher query and returns formatted graph results.
func (t *GraphQueryTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var input GraphQueryInput
	if err := json.Unmarshal(args, &input); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, err
	}

	if input.Cypher == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "cypher query is required",
		}, fmt.Errorf("cypher query is required")
	}

	logger.Infof(ctx, "[Tool][GraphQuery] Executing Cypher: %s", input.Cypher)

	if err := validateGraphCypher(input.Cypher); err != nil {
		logger.Warnf(ctx, "[Tool][GraphQuery] Cypher validation failed: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Cypher validation failed: %v", err),
		}, err
	}

	graphData, err := t.graphRepo.SearchByCypher(ctx, input.Cypher, nil)
	if err != nil {
		logger.Errorf(ctx, "[Tool][GraphQuery] SearchByCypher failed: %v", err)
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("graph query failed: %v", err),
		}, err
	}

	if graphData == nil || len(graphData.Node) == 0 {
		logger.Infof(ctx, "[Tool][GraphQuery] No results")
		return &types.ToolResult{
			Success: true,
			Output:  "Query executed successfully, but returned no nodes.",
			Data: map[string]interface{}{
				"display_type": "graph_query_results",
				"nodes":        0,
				"relations":    0,
			},
		}, nil
	}

	logger.Infof(ctx, "[Tool][GraphQuery] Results: %d nodes, %d relations",
		len(graphData.Node), len(graphData.Relation))

	// Build output text
	var output strings.Builder
	output.WriteString("=== Graph Query Results ===\n\n")
	fmt.Fprintf(&output, "Nodes: %d\n", len(graphData.Node))
	fmt.Fprintf(&output, "Relations: %d\n\n", len(graphData.Relation))

	if len(graphData.Node) > 0 {
		output.WriteString("--- Nodes ---\n")
		for _, node := range graphData.Node {
			line := fmt.Sprintf("  - %s", node.Name)
			if len(node.Attributes) > 0 {
				line += fmt.Sprintf(" [%s]", strings.Join(node.Attributes, ", "))
			}
			line += "\n"
			output.WriteString(line)
		}
		output.WriteString("\n")
	}

	if len(graphData.Relation) > 0 {
		output.WriteString("--- Relations ---\n")
		for _, rel := range graphData.Relation {
			output.WriteString(fmt.Sprintf("  %s -[%s]-> %s\n", rel.Node1, rel.Type, rel.Node2))
		}
		output.WriteString("\n")
	}

	result := output.String()
	const outputLimit = 15000
	if len(result) > outputLimit {
		result = result[:outputLimit]
		result += fmt.Sprintf("\n\n... (output truncated at %d chars, use more specific Cypher to narrow results)", outputLimit)
	}

	return &types.ToolResult{
		Success: true,
		Output:  result,
		Data: map[string]interface{}{
			"graph_data": map[string]interface{}{
				"nodes":       graphData.Node,
				"relations":   graphData.Relation,
				"total_nodes": len(graphData.Node),
				"total_edges": len(graphData.Relation),
			},
			"nodes":        len(graphData.Node),
			"relations":    len(graphData.Relation),
			"display_type": "graph_query_results",
		},
	}, nil
}

// validateGraphCypher checks a Cypher query for dangerous operations before execution.
func validateGraphCypher(cypher string) error {
	upper := strings.ToUpper(cypher)
	trimmed := strings.TrimSpace(upper)

	forbidden := []string{"DELETE", "DETACH", "SET", "CREATE", "MERGE", "DROP", "REMOVE"}
	for _, kw := range forbidden {
		if strings.Contains(upper, kw) {
			return fmt.Errorf("forbidden keyword %q in Cypher query", kw)
		}
	}

	if !strings.HasPrefix(trimmed, "MATCH") && !strings.HasPrefix(trimmed, "CALL") {
		return fmt.Errorf("query must start with MATCH or CALL")
	}

	if strings.Contains(cypher, ";") {
		return fmt.Errorf("multiple statements not allowed")
	}

	return nil
}
