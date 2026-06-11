# Proto API Definitions

服务拆分后，每个领域对应一个 proto 包，定义其数据结构和 gRPC 接口。

## 领域划分

| 包 | 路径 | 职责 |
|---|------|------|
| common | `common/v1/common.proto` | 共享类型：Note, KnowledgeCard, Chunk 等 |
| note | `note/v1/note.proto` | 笔记 CRUD、标签、块、模板、导入导出 |
| intelligence | `intelligence/v1/intelligence.proto` | AI 洞察、复习题、推荐、研究、周报 |
| knowledge | `knowledge/v1/knowledge.proto` | 知识卡片与间隔重复复习 |
| workspace | `workspace/v1/workspace.proto` | 仪表盘、图谱、质量评估 |
| rag | `rag/v1/rag.proto` | RAG 检索、问答、索引 |

## 代码生成（可选）

```bash
# 安装工具
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成 Go 代码
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/*/v1/*.proto
```

> 当前阶段 proto 文件作为 API 契约文档使用，暂不生成 gRPC 代码。
> HTTP/JSON 接口依然通过 `internal/api/` 下的 handler 提供服务。
