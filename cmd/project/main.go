// DB→Neo4j projection CLI.
//
// Reads pending rows from graph_entities/graph_relations and MERGEs them into Neo4j.
//
// Usage:
//
//	go run cmd/project/main.go --kb-id=<uuid> --tenant-id=<uint64>
//	go run cmd/project/main.go --kb-id=<uuid> --tenant-id=<uint64> --entities-only
//	go run cmd/project/main.go --kb-id=<uuid> --tenant-id=<uint64> --relations-only
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/Tencent/WeKnora/internal/container"
	"github.com/Tencent/WeKnora/internal/runtime"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func main() {
	kbID := flag.String("kb-id", os.Getenv("PRIMEKG_KB_ID"), "知识库 ID (UUID)")
	tenantID := flag.Uint64("tenant-id", 0, "租户 ID")
	entitiesOnly := flag.Bool("entities-only", false, "仅投影实体")
	relationsOnly := flag.Bool("relations-only", false, "仅投影关系")
	batchSize := flag.Int("batch", 500, "每批处理数量")

	flag.Parse()

	if *kbID == "" {
		fmt.Fprintln(os.Stderr, "错误: --kb-id 不能为空")
		flag.Usage()
		os.Exit(1)
	}
	if *tenantID == 0 {
		fmt.Fprintln(os.Stderr, "错误: --tenant-id 不能为空")
		flag.Usage()
		os.Exit(1)
	}

	ctx := context.Background()

	c := container.BuildContainer(runtime.GetContainer())

	runEntities := !*relationsOnly
	runRelations := !*entitiesOnly

	err := c.Invoke(func(svc interfaces.GraphProjectionService) {
		if runEntities {
			fmt.Println("开始投影实体...")
			total := 0
			for {
				n, err := svc.ProjectEntities(ctx, *tenantID, *kbID, *batchSize)
				if err != nil {
					fmt.Fprintf(os.Stderr, "实体投影失败: %v\n", err)
					os.Exit(1)
				}
				total += n
				fmt.Printf("本批投影实体: %d 个 (累计: %d)\n", n, total)
				if n == 0 {
					break
				}
			}
			fmt.Printf("实体投影全部完成: %d 个\n", total)
		}

		if runRelations {
			fmt.Println("开始投影关系...")
			total := 0
			for {
				n, err := svc.ProjectRelations(ctx, *tenantID, *kbID, *batchSize)
				if err != nil {
					fmt.Fprintf(os.Stderr, "关系投影失败: %v\n", err)
					os.Exit(1)
				}
				total += n
				fmt.Printf("本批投影关系: %d 个 (累计: %d)\n", n, total)
				if n == 0 {
					break
				}
			}
			fmt.Printf("关系投影全部完成: %d 个\n", total)
		}
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "运行失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("投影完成。")
}
