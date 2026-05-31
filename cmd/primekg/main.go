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

// external_id key → possible Neo4j node_source values (order matters: primary first).
var keyToSources = map[string][]string{
	"drugbank":  {"DrugBank"},
	"mondo":     {"MONDO", "MONDO_grouped"},
	"ncbi_gene": {"NCBI"},
	"reactome":  {"Reactome"},
	"id":        {"MONDO_grouped"},
}

// WeKnora entity type → Neo4j label (matching entity_dict.entity_type values).
var typeToLabel = map[string]string{
	"Drug":       "Drug",
	"Indication": "Indication",
	"Target":     "Target",
	"Pathway":    "Pathway",
}

func main() {
	dryRun := flag.Bool("dry-run", false, "仅预览，不写入 Neo4j")
	flag.Parse()

	ctx := context.Background()
	c := container.BuildContainer(runtime.GetContainer())

	err := c.Invoke(func(db *gorm.DB, driver neo4j.Driver) error {
		if driver == nil {
			return fmt.Errorf("Neo4j 未启用: 请设置环境变量 NEO4J_ENABLE=true 并确保 Neo4j 连接配置正确")
		}
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
				src, id, alt := resolveID(r.ExternalIDs)
				if alt != "" {
					fmt.Printf("  dict:%d  %s  %s|%s:%s\n", r.ID, r.EntityType, src, alt, id)
				} else {
					fmt.Printf("  dict:%d  %s  %s:%s\n", r.ID, r.EntityType, src, id)
				}
			}
			return nil
		}

		// 2. 直接写 Neo4j
		session := driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		defer session.Close(ctx)

		created := 0
		linked := 0
		noType := 0
		noID := 0
		for _, r := range rows {
			label, ok := typeToLabel[r.EntityType]
			if !ok {
				noType++
				if noType <= 3 {
					fmt.Printf("  [跳过] 未知类型 dict:%d entity_type=%q\n", r.ID, r.EntityType)
				}
				continue
			}

			name := ""
			if n, ok := jsonToMap(r.CanonicalData)["name"]; ok {
				name = fmt.Sprintf("%v", n)
			}

			nodeSource, primekgID, altSource := resolveID(r.ExternalIDs)
			if nodeSource == "" || primekgID == "" {
				noID++
				if noID <= 3 {
					fmt.Printf("  [跳过] 无法解析ID dict:%d entity_type=%q external_ids=%s\n", r.ID, r.EntityType, string(r.ExternalIDs))
				}
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

				// REFERENCES 边连到 PrimeKG 原节点; altSource 处理 MONDO/MONDO_grouped
				if altSource != "" {
					_, err = tx.Run(ctx, `
						MATCH (n:GraphEntity {source_entity_id: $seid})
						MATCH (p)
						WHERE p.primekg_id = $pid AND (p.node_source = $src OR p.node_source = $alt)
						MERGE (n)-[:REFERENCES]->(p)
					`, map[string]any{
						"seid": fmt.Sprintf("dict:%d", r.ID),
						"pid":  primekgID,
						"src":  nodeSource,
						"alt":  altSource,
					})
				} else {
					_, err = tx.Run(ctx, `
						MATCH (n:GraphEntity {source_entity_id: $seid})
						MATCH (p {primekg_id: $pid, node_source: $src})
						MERGE (n)-[:REFERENCES]->(p)
					`, map[string]any{
						"seid": fmt.Sprintf("dict:%d", r.ID),
						"pid":  primekgID,
						"src":  nodeSource,
					})
				}
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
		fmt.Printf("完成: 创建 %d 个节点, %d 条 REFERENCES 边 (跳过: 未知类型=%d 无ID=%d)\n", created, linked, noType, noID)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}
}

func resolveID(externalIDs types.JSON) (source, id, altSource string) {
	var extIDs map[string]any
	if err := json.Unmarshal(json.RawMessage(externalIDs), &extIDs); err != nil {
		return
	}
	for key, sources := range keyToSources {
		if v, ok := extIDs[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				if len(sources) >= 2 {
					return sources[0], s, sources[1]
				}
				return sources[0], s, ""
			}
		}
	}
	for k, v := range extIDs {
		if s, ok := v.(string); ok && s != "" {
			return k, s, ""
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
