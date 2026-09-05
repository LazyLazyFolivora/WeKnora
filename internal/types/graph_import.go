package types

// GraphImportRequest is the request body for manual graph import.
type GraphImportRequest struct {
	Nodes     []*GraphNode     `json:"nodes"`
	Relations []*GraphRelation `json:"relations"`
}

// GraphImportResult reports how many graph records were accepted for import.
type GraphImportResult struct {
	ImportedNodes     int `json:"imported_nodes"`
	ImportedRelations int `json:"imported_relations"`
}
