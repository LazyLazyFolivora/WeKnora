// PrimeKG 初始化 CLI: 从 entity_dict 镜像表创建 Neo4j 节点并连接 PrimeKG 原节点。
// 一次性操作，不经过 graph_entities 中间队列。
//
// 用法:
//
//	go run cmd/primekg/main.go
//	go run cmd/primekg/main.go --dry-run
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"

	"github.com/Tencent/WeKnora/internal/container"
	"github.com/Tencent/WeKnora/internal/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// Source → external_id key mapping (consistent with import_primekg_to_db.py).
var sourceToIDKey = map[string]string{
	"DrugBank": "drugbank",
	"MONDO":    "mondo",
	"NCBI":     "ncbi_gene",
	"Reactome": "reactome",
}

// PrimeKG entity type → Neo4j label.
var typeToLabel = map[string]string{
	"drug":         "Drug",
	"disease":      "Indication",
	"gene/protein": "Target",
	"pathway":      "Pathway",
}

func main() {
	dryRun := flag.Bool("dry-run", false, "仅预览，不写入 Neo4j")
	flag.Parse()

	ctx := context.Background()
	c := container.BuildContainer(runtime.GetContainer())

	err := c.Invoke(func(db *gorm.DB, driver neo4j.Driver) error {
		// 1. 读 entity_dict primekg 行
		var rows []types.EntityDictRecord
		if err := db.WithContext(ctx).
			Where("canonical_source = ? AND is_deleted = ?", "primekg", false).
			Find(&rows).Error; err != nil {
			return fmt.Errorf("查询 entity_dict: %w", err)
		}
		fmt.Printf("entity_dict primekg 行: %d\n", len(rows))

		if *dryRun {
			for _, r := range rows {
				src, id := resolveID(r.ExternalIDs)
				fmt.Printf("  dict:%d  %s  %s:%s\n", r.ID, r.EntityType, src, id)
			}
			return nil
		}

		// 2. 直接写 Neo4j
		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		defer session.Close(ctx)

		created := 0
		linked := 0
		for _, r := range rows {
			label, ok := typeToLabel[r.EntityType]
			if !ok {
				continue
			}

			name := ""
			if n, ok := jsonToMap(r.CanonicalData)["name"]; ok {
				name = fmt.Sprintf("%v", n)
			}

			nodeSource, primekgID := resolveID(r.ExternalIDs)
			if nodeSource == "" || primekgID == "" {
				continue
			}

			_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
				// 创建实体副本节点
				_, err := tx.Run(ctx, `
					MERGE (n:GraphEntity {source_entity_id: $seid})
					SET n.entity_type = $type, n.entity_name = $name
				`, map[string]any{
					"seid": fmt.Sprintf("dict:%d", r.ID),
					"type": label,
					"name": name,
				})
				if err != nil {
					return nil, err
				}

				// REFERENCES 边连到 PrimeKG 原节点
				_, err = tx.Run(ctx, `
					MATCH (n:GraphEntity {source_entity_id: $seid})
					MATCH (p {primekg_id: $pid, node_source: $src})
					MERGE (n)-[:REFERENCES]->(p)
				`, map[string]any{
					"seid": fmt.Sprintf("dict:%d", r.ID),
					"pid":  primekgID,
					"src":  nodeSource,
				})
				return nil, err
			})
			if err != nil {
				fmt.Printf("  失败 dict:%d %s: %v\n", r.ID, name, err)
				continue
			}
			created++
			linked++
			if created%500 == 0 {
				fmt.Printf("  已处理 %d/%d\n", created, len(rows))
			}
		}
		fmt.Printf("完成: 创建 %d 个节点, %d 条 REFERENCES 边\n", created, linked)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}

type idPair struct{ source, id string }

func resolveID(externalIDs types.JSON) (source, id string) {
	var extIDs map[string]any
	if err := json.Unmarshal(json.RawMessage(externalIDs), &extIDs); err != nil {
		return
	}
	for src, key := range sourceToIDKey {
		if v, ok := extIDs[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return src, s
			}
		}
	}
	for k, v := range extIDs {
		if s, ok := v.(string); ok && s != "" {
			return k, s
		}
	}
	return
}

func jsonToMap(j types.JSON) map[string]any {
	m := make(map[string]any)
	if len(j) > 2 {
		json.Unmarshal(json.RawMessage(j), &m)
	}
	return m
}
