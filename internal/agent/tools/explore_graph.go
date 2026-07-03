package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
)

var exploreGraphTool = BaseTool{
	name: ToolExploreGraph,
	description: `Deep exploration of a specific entity in the Neo4j knowledge graph. Use AFTER neo4j_hybrid_search has identified relevant entities and you need to understand their neighborhood structure.

## Core Function
Given an entity name, traverses the knowledge graph to discover direct neighbors (1-hop) or hierarchy trees (2-hop), returning entity properties, relationship summaries, and neighborhood structure.

## When to Use
✅ **Use for**:
- Drilling into a known entity (e.g., "tell me more about Aspirin's connections")
- Understanding an entity's neighborhood before answering
- Multi-hop reasoning when you need to see what's connected to intermediate entities
- Exploring local graph structure around a specific node

❌ **Don't use for**:
- Broad search across multiple entities → use neo4j_hybrid_search first
- Pure semantic/conceptual queries → use knowledge_search
- Literal keyword/string matching → use grep_chunks

## Parameters
- **entity** (required): Entity name to explore — use the exact name from previous neo4j_hybrid_search results.
- **relation_filter** (optional): Filter relations by type (e.g., "TREATS", "CAUSES"). Case-insensitive substring match.
- **max_hops** (optional): 1 = direct neighbors only (default), 2 = two-level hierarchy tree with indent-based depth display. Max 2. Use 2 sparingly for complex neighborhoods only.

## How It Works
1. 1-hop: Finds the entity and all directly connected neighbors with their relations (MATCH with CONTAINS)
2. 2-hop: After finding direct neighbors, queries their connections to build an indent-based hierarchy tree
3. Results include entity properties (attributes, entity_type, entity_data) and relation type summaries

## Notes
- Cross-KB search — no per-knowledge-base isolation
- Output truncated at ~15000 chars to fit context window
- Parameterized Cypher only — no LLM generation, always safe for execution`,
	schema: utils.GenerateSchema[ExploreGraphInput](),
}

// ExploreGraphInput defines the input parameters for explore_graph.
type ExploreGraphInput struct {
	Entity         string `json:"entity" jsonschema:"required, Entity name to explore in the knowledge graph"`
	RelationFilter string `json:"relation_filter,omitempty" jsonschema:"Optional filter for relation types (case-insensitive substring match)"`
	MaxHops        int    `json:"max_hops,omitempty" jsonschema:"Traversal depth: 1 = direct neighbors (default), 2 = two-level hierarchy tree. Max 2."`
}

// ExploreGraphTool explores a specific entity's neighborhood in Neo4j.
type ExploreGraphTool struct {
	BaseTool
	graphRepo interfaces.RetrieveGraphRepository
}

// NewExploreGraphTool creates a new explore_graph tool.
func NewExploreGraphTool(graphRepo interfaces.RetrieveGraphRepository) *ExploreGraphTool {
	return &ExploreGraphTool{
		BaseTool:  exploreGraphTool,
		graphRepo: graphRepo,
	}
}

// Execute performs the graph exploration.
func (t *ExploreGraphTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var input ExploreGraphInput
	if err := json.Unmarshal(args, &input); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, err
	}

	if input.Entity == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "entity is required",
		}, fmt.Errorf("entity is required")
	}

	if input.MaxHops <= 0 {
		input.MaxHops = 1
	}
	if input.MaxHops > 2 {
		input.MaxHops = 2
	}

	relationFilter := strings.ToLower(input.RelationFilter)

	logger.Infof(ctx, "[Tool][ExploreGraph] Exploring entity: %s, max_hops: %d, relation_filter: %q",
		input.Entity, input.MaxHops, relationFilter)

	// --- 1-hop query ---
	cypher1 := "MATCH (n)-[r]-(m) WHERE toLower(n.name) CONTAINS toLower($entity) RETURN n, r, m LIMIT 200"
	params1 := map[string]interface{}{"entity": input.Entity}

	graph1, err := t.graphRepo.SearchByCypher(ctx, cypher1, params1)
	if err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("graph search failed: %v", err),
		}, err
	}

	if graph1 == nil || len(graph1.Node) == 0 {
		return &types.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("Entity '%s' not found in the knowledge graph.", input.Entity),
			Data: map[string]interface{}{
				"entity":       input.Entity,
				"max_hops":     input.MaxHops,
				"found":        false,
				"nodes":        0,
				"relations":    0,
				"display_type": "explore_graph_results",
			},
		}, nil
	}

	// Find the primary entity node.
	var primaryNode *types.GraphNode
	for _, node := range graph1.Node {
		if strings.EqualFold(node.Name, input.Entity) {
			primaryNode = node
			break
		}
	}

	// Collect all neighbor names from unfiltered relations (needed for 2-hop).
	allNeighborNames := make(map[string]bool)
	for _, rel := range graph1.Relation {
		if strings.EqualFold(rel.Node1, input.Entity) {
			allNeighborNames[rel.Node2] = true
		} else if strings.EqualFold(rel.Node2, input.Entity) {
			allNeighborNames[rel.Node1] = true
		} else {
			allNeighborNames[rel.Node1] = true
			allNeighborNames[rel.Node2] = true
		}
	}

	// Apply relation filter.
	filteredRelations := make([]*types.GraphRelation, 0, len(graph1.Relation))
	filteredNeighborNames := make(map[string]bool)
	relationTypeCount := make(map[string]int)

	for _, rel := range graph1.Relation {
		if relationFilter != "" && !strings.Contains(strings.ToLower(rel.Type), relationFilter) {
			continue
		}
		filteredRelations = append(filteredRelations, rel)
		relationTypeCount[rel.Type]++

		if strings.EqualFold(rel.Node1, input.Entity) {
			filteredNeighborNames[rel.Node2] = true
		} else if strings.EqualFold(rel.Node2, input.Entity) {
			filteredNeighborNames[rel.Node1] = true
		} else {
			filteredNeighborNames[rel.Node1] = true
			filteredNeighborNames[rel.Node2] = true
		}
	}

	// --- 2-hop query (if requested) ---
	var graph2 *types.GraphData
	neighbor2ndGen := make(map[string][]string) // neighbor name -> depth-2 node names

	if input.MaxHops >= 2 && len(allNeighborNames) > 0 {
		names := make([]string, 0, len(allNeighborNames))
		for name := range allNeighborNames {
			names = append(names, name)
		}
		if len(names) > 50 {
			names = names[:50]
		}

		cypher2 := "MATCH (n)-[r]-(m) WHERE n.name IN $names RETURN n, r, m LIMIT 200"
		params2 := map[string]interface{}{"names": names}

		graph2, err = t.graphRepo.SearchByCypher(ctx, cypher2, params2)
		if err != nil {
			logger.Warnf(ctx, "[Tool][ExploreGraph] 2-hop query failed: %v", err)
			graph2 = nil
		}

		if graph2 != nil {
			seen := make(map[string]bool)
			for _, rel := range graph2.Relation {
				if relationFilter != "" && !strings.Contains(strings.ToLower(rel.Type), relationFilter) {
					continue
				}
				var neighbor, other string
				if allNeighborNames[rel.Node1] && !strings.EqualFold(rel.Node2, input.Entity) {
					neighbor, other = rel.Node1, rel.Node2
				} else if allNeighborNames[rel.Node2] && !strings.EqualFold(rel.Node1, input.Entity) {
					neighbor, other = rel.Node2, rel.Node1
				} else {
					continue
				}
				if allNeighborNames[other] || strings.EqualFold(other, input.Entity) {
					continue
				}
				key := neighbor + "||" + other
				if seen[key] {
					continue
				}
				seen[key] = true
				neighbor2ndGen[neighbor] = append(neighbor2ndGen[neighbor], other)
			}
		}
	}

	// --- Build output ---
	var output strings.Builder
	output.WriteString("=== Explore Graph Results ===\n\n")
	fmt.Fprintf(&output, "Entity: %s\n", input.Entity)
	fmt.Fprintf(&output, "Max Hops: %d\n\n", input.MaxHops)

	// Entity properties
	output.WriteString("--- Entity Properties ---\n")
	if primaryNode != nil && len(primaryNode.Attributes) > 0 {
		for _, attr := range primaryNode.Attributes {
			fmt.Fprintf(&output, "  - %s\n", attr)
		}
	} else {
		output.WriteString("  (no attributes)\n")
	}
	if primaryNode != nil {
		fmt.Fprintf(&output, "  Chunks: %d associated\n", len(primaryNode.Chunks))
	}
	output.WriteString("\n")

	// Relation type summary
	output.WriteString("--- Relation Types ---\n")
	if len(relationTypeCount) > 0 {
		type rc struct {
			typ   string
			count int
		}
		sorted := make([]rc, 0, len(relationTypeCount))
		for typ, count := range relationTypeCount {
			sorted = append(sorted, rc{typ, count})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

		for _, item := range sorted {
			fmt.Fprintf(&output, "  - %s: %d\n", item.typ, item.count)
		}
	} else {
		output.WriteString("  (no relations match filter)\n")
	}
	output.WriteString("\n")

	if input.MaxHops == 1 {
		// 1-hop: flat relation list
		output.WriteString("--- Direct Relations ---\n")
		for _, rel := range filteredRelations {
			fmt.Fprintf(&output, "  %s -[%s]-> %s\n", rel.Node1, rel.Type, rel.Node2)
		}
	} else {
		// 2-hop: indent hierarchy tree
		output.WriteString("--- Hierarchy Tree ---\n")
		fmt.Fprintf(&output, "  %s (depth 0)\n", input.Entity)

		// Build a node lookup for quick attribute access.
		nodeLookup := make(map[string]*types.GraphNode, len(graph1.Node))
		for _, node := range graph1.Node {
			nodeLookup[strings.ToLower(node.Name)] = node
		}
		if graph2 != nil {
			for _, node := range graph2.Node {
				key := strings.ToLower(node.Name)
				if _, exists := nodeLookup[key]; !exists {
					nodeLookup[key] = node
				}
			}
		}

		printed := make(map[string]bool)
		for _, rel := range filteredRelations {
			var neighborName string
			if strings.EqualFold(rel.Node1, input.Entity) {
				neighborName = rel.Node2
			} else {
				neighborName = rel.Node1
			}
			if printed[neighborName] {
				continue
			}
			printed[neighborName] = true

			nn := nodeLookup[strings.ToLower(neighborName)]
			attrs := nodeAttrs(nn)
			fmt.Fprintf(&output, "  ├─ %s (depth 1)%s\n", neighborName, attrs)

			children := neighbor2ndGen[neighborName]
			sort.Strings(children)
			for _, child := range children {
				cn := nodeLookup[strings.ToLower(child)]
				childAttrs := nodeAttrs(cn)
				fmt.Fprintf(&output, "  │  └─ %s (depth 2)%s\n", child, childAttrs)
			}
		}
	}

	// Summary
	depth2Count := 0
	for _, children := range neighbor2ndGen {
		depth2Count += len(children)
	}

	output.WriteString("\n--- Summary ---\n")
	fmt.Fprintf(&output, "  Depth-1 neighbors: %d\n", len(filteredNeighborNames))
	fmt.Fprintf(&output, "  Depth-1 relations: %d\n", len(filteredRelations))
	fmt.Fprintf(&output, "  Depth-2 nodes: %d\n", depth2Count)
	output.WriteString("\n")

	result := output.String()
	const outputLimit = 15000
	if len(result) > outputLimit {
		result = result[:outputLimit]
		result += fmt.Sprintf("\n\n... (output truncated at %d chars)", outputLimit)
	}

	// Build Data payload
	allNodes := graph1.Node
	allRelations := filteredRelations
	if graph2 != nil {
		allNodes = append(allNodes, graph2.Node...)
		allRelations = append(allRelations, graph2.Relation...)
	}

	return &types.ToolResult{
		Success: true,
		Output:  result,
		Data: map[string]interface{}{
			"entity":          input.Entity,
			"max_hops":        input.MaxHops,
			"relation_filter": input.RelationFilter,
			"found":           true,
			"graph_data": map[string]interface{}{
				"nodes":       allNodes,
				"relations":   allRelations,
				"total_nodes": len(allNodes),
				"total_edges": len(allRelations),
			},
			"nodes":        len(allNodes),
			"relations":    len(allRelations),
			"display_type": "explore_graph_results",
		},
	}, nil
}

// nodeAttrs formats a node's attributes for display.
func nodeAttrs(node *types.GraphNode) string {
	if node == nil || len(node.Attributes) == 0 {
		return ""
	}
	return " [" + strings.Join(node.Attributes, ", ") + "]"
}
