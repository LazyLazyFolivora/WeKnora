package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
	"golang.org/x/sync/errgroup"
)

var neo4jHybridSearchTool = BaseTool{
	name: ToolNeo4jHybridSearch,
	description: `Hybrid search combining Neo4j knowledge graph traversal with vector/keyword chunk retrieval.

## Core Function
Runs Neo4j graph search and vector/keyword search in parallel across all configured knowledge bases, then merges and deduplicates results. This gives the model both entity-relationship knowledge from the graph AND semantic/lexical matches from chunk indexes.

## When to Use
✅ **Use for**:
- Understanding relationships between entities (e.g., "how does X relate to Y?")
- Exploring knowledge networks and concept associations
- Queries that need both structural context (graph) and detailed content (chunks)
- Multi-hop questions where entity connections matter

❌ **Don't use for**:
- Pure semantic/conceptual queries → use knowledge_search
- Literal keyword/string matching → use grep_chunks
- Knowledge bases without graph extraction configured

## Parameters
- **query** (required): Natural language search query — used for both graph node matching and vector/keyword retrieval.

## How It Works
1. Neo4j graph search: LLM extracts entities from query, generates Cypher query, executes against Neo4j
2. Chunk search: vector + keyword hybrid retrieval across chunk indexes
3. Both run in parallel, results are merged, deduplicated by chunk ID, and sorted by relevance

## Notes
- Searches across all configured knowledge bases
- Graph search is cross-document within each KB (no per-document isolation)
- Results include graph structure data for visualization when applicable`,
	schema: utils.GenerateSchema[Neo4jHybridSearchInput](),
}

// Neo4jHybridSearchInput defines the input parameters for neo4j_hybrid_search.
type Neo4jHybridSearchInput struct {
	Query string `json:"query" jsonschema:"Search query for both graph traversal and vector/keyword retrieval"`
}

// Neo4jHybridSearchTool combines Neo4j graph traversal with vector/keyword hybrid search.
type Neo4jHybridSearchTool struct {
	BaseTool
	graphRepo            interfaces.RetrieveGraphRepository
	knowledgeBaseService interfaces.KnowledgeBaseService
	chunkRepo            interfaces.ChunkRepository
	knowledgeRepo        interfaces.KnowledgeRepository
	searchTargets        types.SearchTargets
	chatModel            chat.Chat
}

// NewNeo4jHybridSearchTool creates a new neo4j_hybrid_search tool.
func NewNeo4jHybridSearchTool(
	graphRepo interfaces.RetrieveGraphRepository,
	knowledgeBaseService interfaces.KnowledgeBaseService,
	chunkRepo interfaces.ChunkRepository,
	knowledgeRepo interfaces.KnowledgeRepository,
	searchTargets types.SearchTargets,
	chatModel chat.Chat,
) *Neo4jHybridSearchTool {
	return &Neo4jHybridSearchTool{
		BaseTool:             neo4jHybridSearchTool,
		graphRepo:            graphRepo,
		knowledgeBaseService: knowledgeBaseService,
		chunkRepo:            chunkRepo,
		knowledgeRepo:        knowledgeRepo,
		searchTargets:        searchTargets,
		chatModel:            chatModel,
	}
}

// Execute performs the hybrid Neo4j graph + vector/keyword search across all KBs.
func (t *Neo4jHybridSearchTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var input Neo4jHybridSearchInput
	if err := json.Unmarshal(args, &input); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, err
	}

	if input.Query == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "query is required",
		}, fmt.Errorf("query is required")
	}

	if len(t.searchTargets) == 0 {
		return &types.ToolResult{
			Success: true,
			Output:  "No knowledge bases configured for hybrid search.",
		}, nil
	}

	kbIDs := t.searchTargets.GetAllKnowledgeBaseIDs()
	logger.Infof(ctx, "[Tool][Neo4jHybridSearch] Querying %d KBs: %v", len(kbIDs), kbIDs)

	// Pre-scan KBs: collect graph schema, tenantID, and capability flags
	var tenantID uint64
	hasChunkAny := false
	var graphSchemas []graphSchemaInfo
	for _, target := range t.searchTargets {
		if tenantID == 0 {
			tenantID = target.TenantID
		}
		kb, err := t.knowledgeBaseService.GetKnowledgeBaseByID(ctx, target.KnowledgeBaseID)
		if err != nil {
			continue
		}
		// Always collect graph schema — the graph may be independently built without ExtractConfig
		graphSchemas = append(graphSchemas, graphSchemaInfo{
			kbID:     target.KnowledgeBaseID,
			config:   kb.ExtractConfig,
			tenantID: target.TenantID,
		})
		if kb.IsVectorEnabled() || kb.IsKeywordEnabled() {
			hasChunkAny = true
		}
	}

	var (
		mu           sync.Mutex
		allResults   []*types.SearchResult
		allNodes     []*types.GraphNode
		allRelations []*types.GraphRelation
		errors       []string
	)

	g, gCtx := errgroup.WithContext(ctx)

	// Graph search: generate Cypher (LLM or deterministic), validate, execute
	g.Go(func() error {
		var cypher string
		var err error

		if t.chatModel != nil {
			cypher, err = t.generateCypherQuery(gCtx, input.Query, graphSchemas)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("Cypher generation failed: %v", err))
				mu.Unlock()
				return nil
			}
		} else {
			// Deterministic fallback: plain CONTAINS search, no LLM needed
			cypher = "MATCH (n)-[r]-(m) WHERE toLower(n.name) CONTAINS toLower($search_term) RETURN n, r, m LIMIT 50"
		}

		if cypher == "" {
			return nil
		}

		if err := validateCypher(cypher); err != nil {
			mu.Lock()
			errors = append(errors, fmt.Sprintf("Cypher validation failed: %v", err))
			mu.Unlock()
			return nil
		}

		logger.Infof(gCtx, "[Tool][Neo4jHybridSearch] Executing Cypher: %s", cypher)

		params := map[string]interface{}{"search_term": input.Query}
		graphData, err := t.graphRepo.SearchByCypher(gCtx, cypher, params)
		if err != nil {
			mu.Lock()
			errors = append(errors, fmt.Sprintf("graph search failed: %v", err))
			mu.Unlock()
			return nil
		}
		if graphData == nil || len(graphData.Node) == 0 {
			return nil
		}

		mu.Lock()
		allNodes = append(allNodes, graphData.Node...)
		allRelations = append(allRelations, graphData.Relation...)
		mu.Unlock()

		chunkIDs := extractChunkIDs(graphData.Node)
		if len(chunkIDs) > 0 {
			chunks, err := t.chunkRepo.ListChunksByIDOnly(gCtx, chunkIDs)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("failed to load graph chunks: %v", err))
				mu.Unlock()
				return nil
			}

			knowledgeIDs := make([]string, 0, len(chunks))
			for _, c := range chunks {
				knowledgeIDs = append(knowledgeIDs, c.KnowledgeID)
			}

			knowledges, err := t.knowledgeRepo.GetKnowledgeBatch(gCtx, tenantID, knowledgeIDs)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("failed to load knowledge: %v", err))
				mu.Unlock()
				return nil
			}

			knowledgeMap := make(map[string]*types.Knowledge, len(knowledges))
			for _, k := range knowledges {
				knowledgeMap[k.ID] = k
			}

			mu.Lock()
			for _, chunk := range chunks {
				if k, ok := knowledgeMap[chunk.KnowledgeID]; ok {
					allResults = append(allResults, graphChunkToSearchResult(chunk, k))
				}
			}
			mu.Unlock()
		}
		return nil
	})

	// Chunk search: per KB
	if hasChunkAny {
		for _, target := range t.searchTargets {
			target := target
			g.Go(func() error {
				kb, err := t.knowledgeBaseService.GetKnowledgeBaseByID(gCtx, target.KnowledgeBaseID)
				if err != nil || (!kb.IsVectorEnabled() && !kb.IsKeywordEnabled()) {
					return nil
				}
				searchParams := types.SearchParams{
					QueryText:  input.Query,
					MatchCount: 10,
				}
				results, err := t.knowledgeBaseService.HybridSearch(gCtx, target.KnowledgeBaseID, searchParams)
				if err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("KB %s: chunk search failed: %v", target.KnowledgeBaseID, err))
					mu.Unlock()
					return nil
				}
				mu.Lock()
				allResults = append(allResults, results...)
				mu.Unlock()
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Search failed: %v", err),
		}, err
	}

	// Deduplicate by chunk ID
	seen := make(map[string]*types.SearchResult, len(allResults))
	for _, r := range allResults {
		if r == nil {
			continue
		}
		if existing, ok := seen[r.ID]; !ok || r.Score > existing.Score {
			seen[r.ID] = r
		}
	}

	deduped := make([]*types.SearchResult, 0, len(seen))
	for _, r := range seen {
		deduped = append(deduped, r)
	}

	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].Score > deduped[j].Score
	})

	if len(deduped) == 0 && len(allNodes) == 0 {
		return &types.ToolResult{
			Success: true,
			Output:  "No relevant results found from graph or chunk search.",
			Data: map[string]interface{}{
				"query":           input.Query,
				"knowledge_bases": kbIDs,
				"results":         []interface{}{},
				"graph_nodes":     len(allNodes),
				"graph_relations": len(allRelations),
				"errors":          errors,
			},
		}, nil
	}

	// Build output
	output := "=== Neo4j Hybrid Search Results ===\n\n"
	output += fmt.Sprintf("Query: %s\n", input.Query)
	output += fmt.Sprintf("Knowledge Bases: %v\n", kbIDs)
	output += fmt.Sprintf("Results: %d chunks (deduplicated)\n", len(deduped))
	output += fmt.Sprintf("Graph: %d nodes, %d relations\n\n", len(allNodes), len(allRelations))

	// Include graph node/relation details as text so the LLM can read them
	// even when nodes have no associated chunks
	if len(allNodes) > 0 {
		output += "=== Graph Nodes ===\n"
		for _, node := range allNodes {
			line := fmt.Sprintf("  - %s", node.Name)
			if len(node.Attributes) > 0 {
				line += fmt.Sprintf(" [%s]", strings.Join(node.Attributes, ", "))
			}
			line += "\n"
			output += line
		}
		output += "\n"
	}
	if len(allRelations) > 0 {
		output += "=== Graph Relations ===\n"
		for _, rel := range allRelations {
			output += fmt.Sprintf("  - %s -[%s]-> %s\n", rel.Node1, rel.Type, rel.Node2)
		}
		output += "\n"
	}

	if len(errors) > 0 {
		output += "=== Partial Errors ===\n"
		for _, errMsg := range errors {
			output += fmt.Sprintf("  - %s\n", errMsg)
		}
		output += "\n"
	}

	formattedResults := make([]map[string]interface{}, 0, len(deduped))
	for i, r := range deduped {
		relevance := GetRelevanceLevel(r.Score)
		output += fmt.Sprintf("Result #%d:\n", i+1)
		output += fmt.Sprintf("  Relevance: %.2f (%s)\n", r.Score, relevance)
		output += fmt.Sprintf("  Match Type: %s\n", FormatMatchType(r.MatchType))
		output += fmt.Sprintf("  Source: %s\n", r.KnowledgeTitle)
		output += fmt.Sprintf("  Content: %s\n", r.Content)
		output += fmt.Sprintf("  chunk_id: %s\n\n", r.ID)

		formattedResults = append(formattedResults, map[string]interface{}{
			"result_index":    i + 1,
			"chunk_id":        r.ID,
			"content":         r.Content,
			"score":           r.Score,
			"relevance_level": relevance,
			"knowledge_id":    r.KnowledgeID,
			"knowledge_title": r.KnowledgeTitle,
			"match_type":      FormatMatchType(r.MatchType),
		})
	}

	graphData := map[string]interface{}{
		"nodes":       allNodes,
		"relations":   allRelations,
		"total_nodes": len(allNodes),
		"total_edges": len(allRelations),
	}

	// Truncate output if too long (~15000 char soft limit)
	const outputLimit = 15000
	if len(output) > outputLimit {
		output = output[:outputLimit]
		output += fmt.Sprintf("\n\n... (output truncated at %d chars, use explore_graph to drill down)", outputLimit)
	}

	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data: map[string]interface{}{
			"query":           input.Query,
			"knowledge_bases": kbIDs,
			"results":         formattedResults,
			"count":           len(deduped),
			"graph_data":      graphData,
			"graph_nodes":     len(allNodes),
			"graph_relations": len(allRelations),
			"errors":          errors,
			"display_type":    "neo4j_hybrid_search_results",
		},
	}, nil
}

// graphSchemaInfo holds the graph schema extracted from a KB's ExtractConfig.
type graphSchemaInfo struct {
	kbID     string
	config   *types.ExtractConfig
	tenantID uint64
}

// generateCypherQuery calls the LLM to extract entities and generate a Cypher query from the user's natural language input.
func (t *Neo4jHybridSearchTool) generateCypherQuery(
	ctx context.Context,
	query string,
	schemas []graphSchemaInfo,
) (string, error) {
	var schemaLines []string
	for _, s := range schemas {
		if s.config == nil {
			continue
		}
		nodeTypes := make([]string, len(s.config.Nodes))
		for i, n := range s.config.Nodes {
			nodeTypes[i] = n.Name
		}
		relTypes := make([]string, len(s.config.Relations))
		for i, r := range s.config.Relations {
			relTypes[i] = r.Type
		}
		schemaLines = append(schemaLines, fmt.Sprintf(
			"KB %s: entity_types=%v, relation_types=%v",
			s.kbID, nodeTypes, relTypes,
		))
	}

	systemPrompt := fmt.Sprintf(`You are a Cypher query generator for a Neo4j knowledge graph. Generate a read-only MATCH query based on the user's natural language input.

## Graph Schema
%s

## Rules
- Do NOT use any node labels — use plain (n)-[r]-(m) to search across all KBs.
- Node properties: name, chunks, attributes, entity_type, entity_data.
- Use toLower(n.name) CONTAINS toLower($search_term) for case-insensitive name matching.
- Always add LIMIT 50 to avoid scanning the entire graph.
- The query MUST RETURN n, r, m (source node, relationship, target node).
- Output ONLY the Cypher query, no explanation, no markdown.`, strings.Join(schemaLines, "\n"))

	messages := []chat.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: query},
	}

	resp, err := t.chatModel.Chat(ctx, messages, &chat.ChatOptions{Temperature: 0.1})
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	cypher := strings.TrimSpace(resp.Content)
	cypher = strings.TrimPrefix(cypher, "```cypher")
	cypher = strings.TrimPrefix(cypher, "```")
	cypher = strings.TrimSuffix(cypher, "```")
	cypher = strings.TrimSpace(cypher)

	if cypher == "" || !strings.HasPrefix(strings.ToUpper(cypher), "MATCH") {
		return "", fmt.Errorf("LLM returned invalid Cypher: %s", cypher)
	}

	return cypher, nil
}

// validateCypher checks a LLM-generated Cypher query for dangerous operations before execution.
func validateCypher(cypher string) error {
	upper := strings.ToUpper(cypher)

	forbidden := []string{"DELETE", "DETACH", "SET", "CREATE", "MERGE", "DROP", "REMOVE"}
	for _, kw := range forbidden {
		if strings.Contains(upper, kw) {
			return fmt.Errorf("forbidden keyword %q in Cypher query", kw)
		}
	}

	if !strings.HasPrefix(strings.TrimSpace(upper), "MATCH") {
		return fmt.Errorf("query must start with MATCH")
	}

	if strings.Contains(cypher, ";") {
		return fmt.Errorf("multiple statements not allowed")
	}

	return nil
}

// extractChunkIDs collects unique chunk IDs from graph nodes.
func extractChunkIDs(nodes []*types.GraphNode) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, node := range nodes {
		for _, chunkID := range node.Chunks {
			if chunkID == "" || seen[chunkID] {
				continue
			}
			seen[chunkID] = true
			ids = append(ids, chunkID)
		}
	}
	return ids
}

// graphChunkToSearchResult converts a chunk loaded from graph node references into a SearchResult.
func graphChunkToSearchResult(chunk *types.Chunk, knowledge *types.Knowledge) *types.SearchResult {
	return &types.SearchResult{
		ID:                chunk.ID,
		Content:           chunk.Content,
		KnowledgeID:       chunk.KnowledgeID,
		ChunkIndex:        chunk.ChunkIndex,
		KnowledgeTitle:    knowledge.Title,
		StartAt:           chunk.StartAt,
		EndAt:             chunk.EndAt,
		Seq:               chunk.ChunkIndex,
		Score:             1.0,
		MatchType:         types.MatchTypeGraph,
		Metadata:          knowledge.GetMetadata(),
		ChunkType:         string(chunk.ChunkType),
		ParentChunkID:     chunk.ParentChunkID,
		ImageInfo:         chunk.ImageInfo,
		KnowledgeFilename: knowledge.FileName,
		KnowledgeSource:   knowledge.Source,
		KnowledgeChannel:  knowledge.Channel,
		ChunkMetadata:     chunk.Metadata,
		KnowledgeBaseID:   knowledge.KnowledgeBaseID,
	}
}
