package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	_ "modernc.org/sqlite"

	"note/internal/llm"
	"note/internal/rag"
	"note/internal/store"
)

type seedDoc struct {
	Key       string
	Title     string
	FolderKey string
	Tags      []string
	Markdown  string
}

func main() {
	ctx := context.Background()
	dsn := getenv("APP_DSN", "file:notes.db?_pragma=busy_timeout(5000)")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	st := store.New(db)
	if err := st.InitSchema(ctx); err != nil {
		log.Fatalf("init schema: %v", err)
	}

	md := goldmark.New()
	indexer := newIndexer(st)

	folders := []seedDoc{
		{
			Key:      "backend_folder",
			Title:    "后端工程",
			Tags:     []string{"folder", "catalog"},
			Markdown: "# 后端工程\n\n这个目录收纳 Go、HTTP、数据库与检索相关页面，适合演示分层管理与页面跳转。",
		},
		{
			Key:      "ai_folder",
			Title:    "AI 与检索",
			Tags:     []string{"folder", "catalog"},
			Markdown: "# AI与检索\n\n这个目录收纳 RAG、向量化与全文检索相关页面。",
		},
		{
			Key:      "frontend_folder",
			Title:    "前端体验",
			Tags:     []string{"folder", "catalog"},
			Markdown: "# 前端体验\n\n这个目录收纳 Vue 性能优化、交互设计与演示总览。",
		},
	}

	pages := []seedDoc{
		{
			Key:       "go_gc",
			Title:     "Go GC 原理详解",
			FolderKey: "backend_folder",
			Tags:      []string{"go", "runtime", "gc"},
			Markdown: strings.TrimSpace(`
# Go GC 原理详解

Go 的垃圾回收器是并发标记-清扫（mark-sweep）方案，核心目标是把吞吐、内存占用与暂停时间平衡在一个工程可接受的区间。官方 GC 指南明确指出，GC 的主要成本来自两部分：一是内存成本（存活堆 + 新分配堆），二是 CPU 成本（每轮固定开销 + 与 live heap 成比例的扫描开销）。这意味着我们做调优时，不应该只盯着“频率高不高”，而应该把分配速率、对象存活率、指针密度和服务延迟目标一起看。

在实践中最常用的旋钮是 GOGC。可以把它理解为“下一轮 GC 前允许新分配堆增长到什么程度”。GOGC 提高，通常会减少 GC 触发频率，CPU 压力下降，但峰值内存上升；GOGC 降低则相反。官方文档给出的直觉是：在大多数稳态场景中，把 GOGC 大致翻倍，GC CPU 占比可能接近减半，但内存开销会上升。这不是绝对公式，而是一个很实用的估算起点。我们做容量评估时，建议先选一个保守值（如 100），结合 pprof、gctrace 和业务 SLA 逐步迭代。

另一个常被忽视的点是“并发正确性影响回收质量”。Go 内存模型强调：并发读写共享数据必须通过 channel、mutex 或 atomic 串行化。若出现数据竞争，程序语义本身就不可靠，GC 观察到的对象可达性也可能偏离预期，最终表现为“偶发泄漏”“偶发高延迟”这类难复现问题。所以生产中应把 go test -race、压力回放与 GC 指标联动起来看。

## 调优清单

- 先测分配速率和对象生命周期，再谈 GOGC；
- 大对象和短命对象分池处理，避免巨型瞬时分配；
- 用 pprof 看热点分配函数，而不是盲目“全局调参数”；
- 对高并发链路优先清理不必要的指针字段，降低扫描成本。

## 示例

~~~go
// 伪代码：按请求粒度复用缓冲区，减少短命分配
buf := pool.Get().(*bytes.Buffer)
buf.Reset()
defer pool.Put(buf)
~~~

## 关联页面

- [Go 并发流水线与取消机制]({{go_pipeline}})
- [HTTP API 语义与错误处理]({{http_api}})

## 参考资料

- https://go.dev/doc/gc-guide
- https://go.dev/ref/mem
- https://go.dev/blog/pipelines
`),
		},
		{
			Key:       "go_pipeline",
			Title:     "Go 并发流水线与取消机制",
			FolderKey: "backend_folder",
			Tags:      []string{"go", "concurrency"},
			Markdown: strings.TrimSpace(`
# Go 并发流水线与取消机制

Go 官方博客在 pipelines 文章里给出一个非常实用的思路：把复杂任务拆成多个 stage，每个 stage 通过 channel 连接，形成可组合的数据流。这个模式的优势不是“代码看起来并发”，而是可以清晰定义每个阶段的输入输出、背压点与失败边界。尤其在数据预处理、批量导入、日志聚合、内容索引等场景，流水线比“单个大函数 + 多层 if/for”更容易扩展和排障。

真正影响线上稳定性的，是取消和收尾。官方示例强调，如果下游提前退出，上游 goroutine 可能卡在发送操作，导致 goroutine 泄漏。解决方法是引入 done channel（或 context），所有 stage 在发送/接收时都要监听取消信号。这样一旦某个阶段判定“无需继续”，能快速让上游释放资源。这种机制和 HTTP 请求上下文天然兼容：当客户端断开或超时，context 取消向下游传播，数据库查询、远程调用、序列化过程都可以及时停下。

另外要注意“有界并行”。文章里对大目录做哈希计算时，如果每个文件都起一个 goroutine，在大规模输入下会造成内存飙升。更稳妥的做法是固定 worker 数量，用任务队列喂给 worker。这样吞吐虽不一定最高，但可预测性明显更好。工程实践通常会把 worker 数、队列长度和超时策略做成配置项，并结合监控动态调整。

## 工程建议

- stage 之间只传递必要字段，避免对象过重；
- 统一使用 context 作为取消与超时入口；
- 对 fan-out/fan-in 结构加指标：队列长度、处理耗时、错误率；
- 先做可观测性再做并发优化，否则很难判断收益。

## 关联页面

- [Go GC 原理详解]({{go_gc}})
- [SQLite 索引与查询规划]({{sqlite_index}})

## 参考资料

- https://go.dev/blog/pipelines
- https://go.dev/ref/mem
`),
		},
		{
			Key:       "http_api",
			Title:     "HTTP API 语义与错误处理",
			FolderKey: "backend_folder",
			Tags:      []string{"http", "api", "backend"},
			Markdown: strings.TrimSpace(`
# HTTP API 语义与错误处理

很多系统接口“能用但不好维护”，常见原因不是框架问题，而是语义不一致：同一类错误在不同接口里返回不同状态码，删除和归档行为混在一起，重试策略没有幂等保证。RFC 9110 给了我们统一语义基线：GET 用于读取、POST 用于创建、PUT 用于整体更新、PATCH 用于部分更新、DELETE 用于删除当前表示。只要围绕这个基线建立团队规范，前后端协作会顺畅很多。

状态码方面，建议坚持“问题在哪一层，就在哪一层表达”。参数格式不对是 400，资源不存在是 404，方法不允许是 405，服务器未实现是 501，网关依赖失败可用 502/504。不要把所有错误都压成 200 + 业务码，这会让缓存、中间件、监控和重试逻辑都失去意义。对于可恢复失败（例如外部服务超时），要明确是否可重试、重试窗口多长、是否需要幂等键。

在笔记系统里，archive 与 delete 就是一个典型例子：归档是软删除，仍可恢复；永久删除才是 DELETE。如果客户端要展示“垃圾箱”，它应调用“仅归档项列表”接口，而不是在全量列表里自行过滤。这样服务端可逐步加入策略，比如归档 30 天自动清理、审计日志补偿等，而不破坏客户端行为。

最后，错误响应结构应稳定：error（人类可读）、code（机器可判定）、request_id（链路追踪）是最常见最有价值的三件套。把这些规范固化到中间件里，比在每个 handler 手写更可靠。

## 关联页面

- [RAG 系统设计与落地清单]({{rag_design}})
- [SQLite 索引与查询规划]({{sqlite_index}})

## 参考资料

- https://www.rfc-editor.org/rfc/rfc9110
`),
		},
		{
			Key:       "sqlite_index",
			Title:     "SQLite 索引与查询规划",
			FolderKey: "data_folder",
			Tags:      []string{"sqlite", "database"},
			Markdown: strings.TrimSpace(`
# SQLite 索引与查询规划

SQLite 官方文档把 query planner 描述成“在众多等价算法中挑一个更快方案的 AI”。这句话很重要：SQL 是声明式语言，我们描述“要什么”，而不是“怎么做”。但 planner 想选出好计划，前提是你提供了可用索引。没有索引时，查询只能全表扫描；数据量一大，延迟会线性上升。对演示系统来说，早期就规划索引，能避免后面出现“页面越来越卡”的被动局面。

文档对成本的解释很清楚：基于 rowid 或索引的查找通常接近二分搜索，复杂度更像 logN；全表扫描是 N。更进一步，复合索引能把多条件查询压缩成更少的搜索步骤，例如 (fruit, state) 比单列索引更适合 WHERE fruit=? AND state=?。还有一个高频经验：若一个索引是另一个索引的前缀，通常保留更长那个即可，避免冗余写放大。

排序同样受益于索引。ORDER BY 如果命中索引顺序，可以少做甚至不做额外排序；覆盖索引还能减少回表，提高吞吐并降低临时存储压力。这对笔记搜索列表尤其关键：我们常用 updated_at DESC，若有合适索引，首页和搜索弹窗就能更稳定地给出结果。

全文场景下，SQLite 提供 FTS5 扩展，支持 MATCH、短语、前缀、NEAR 与自定义排序函数。对本地应用而言，FTS5 部署成本低，不依赖外部服务；当数据规模进一步扩大，才需要考虑引入 Elasticsearch 这类分布式检索引擎。演示阶段建议先把“可用性与结构化索引”做扎实，再谈复杂集群能力。

## 关联页面

- [RAG 系统设计与落地清单]({{rag_design}})
- [Elasticsearch 检索建模实战]({{es_practice}})

## 参考资料

- https://www.sqlite.org/queryplanner.html
- https://www.sqlite.org/fts5.html
`),
		},
		{
			Key:       "rag_design",
			Title:     "RAG 系统设计与落地清单",
			FolderKey: "ai_folder",
			Tags:      []string{"rag", "llm", "retrieval"},
			Markdown: strings.TrimSpace(`
# RAG 系统设计与落地清单

RAG（Retrieval-Augmented Generation）的核心思想，是把“参数记忆”与“外部可更新知识”结合起来。NeurIPS 2020 论文指出，纯参数化模型在知识密集任务上会受限：知识更新慢、证据链弱、可解释性不足。RAG 通过检索器从外部语料取回相关片段，再交给生成器完成回答，能够在事实性和可追溯性上取得更好平衡。

工程上可以把 RAG 分成四段：数据入库、切块、向量化、检索生成。数据入库阶段强调“结构化元数据”，例如来源、时间、标签、权限；切块阶段要控制 chunk 长度与语义完整性，避免一段里混入多个主题；向量化阶段依赖 embedding 模型，把文本映射到向量空间；在线查询阶段做相似度检索（可混合关键词过滤），再把 top-k 证据拼接进提示词，交给生成模型回答。每一段都要有可观测性：命中率、重排收益、答案可用率、拒答率、延迟分位数。

很多团队失败在“只搭链路，不做评估”。建议至少维护三类评测集：事实问答、步骤型问答、跨文档综合问答；并记录每次回答使用了哪些片段，以便回放与修订。对于敏感场景，还要显式输出“不确定”并给出处，降低幻觉风险。若语料更新频繁，离线全量重建向量成本高，可以采用增量索引策略：新文档优先入库，热数据优先重算，低频历史分批处理。

## 实施清单

- 定义语料标准：来源可信、字段齐全、可追踪；
- 先做检索质量基线，再上复杂提示工程；
- 强制记录上下文片段，支持审计与回放；
- 建立“答案正确率 + 时延 + 成本”三维看板。

## 关联页面

- [SQLite 索引与查询规划]({{sqlite_index}})
- [Elasticsearch 检索建模实战]({{es_practice}})
- [HTTP API 语义与错误处理]({{http_api}})

## 参考资料

- https://proceedings.neurips.cc/paper/2020/hash/6b493230205f780e1bc26945df7481e5-Abstract.html
- https://arxiv.org/abs/2005.11401
- https://platform.openai.com/docs/api-reference/embeddings
`),
		},
		{
			Key:       "es_practice",
			Title:     "Elasticsearch 检索建模实战",
			FolderKey: "ai_folder",
			Tags:      []string{"elasticsearch", "search"},
			Markdown: strings.TrimSpace(`
# Elasticsearch 检索建模实战

当数据规模、并发和跨字段查询复杂度上来后，分布式检索系统会比单机数据库更有优势。Elasticsearch 的核心能力是倒排索引 + 分片并行 + 丰富查询 DSL。简单理解：每个词项会映射到文档列表，查询时先命中词项再回收候选文档，最后计算排序分数。这个机制天然适合全文搜索、过滤组合、聚合统计和在线推荐场景。

实战里最关键的是“先建模，再写查询”。字段类型（text/keyword/date/numeric）直接决定检索行为，分析器（analyzer）决定分词与归一化策略。比如标题通常需要全文匹配（text）并保留精确匹配子字段（keyword），标签、状态、作者这类过滤字段应优先用 keyword。若一开始字段设计混乱，后期补救成本很高，往往需要重建索引。

查询层面建议采用“过滤与相关性分离”：必须条件放 filter（不计分、可缓存），语义相关部分放 query（参与评分）。当结果解释性要求高时，可开启 profile 分析瓶颈；当需要规则干预时，可使用 query rules 做置顶或排除。线上系统还要关注分片数量与路由策略，避免“小分片过多”导致调度开销高于查询收益。很多性能问题不是算法差，而是索引生命周期管理不到位，比如冷热分层、过期数据清理、mapping 漂移等。

在本项目阶段，我们可以把 Elasticsearch 作为“增强检索”的未来方案：先用 SQLite FTS5 和向量检索打稳最小闭环，再根据语料规模与 QPS 决定是否引入 ES。这样不会把演示复杂度拉爆，也能给后续扩展留清晰路径。

## 关联页面

- [RAG 系统设计与落地清单]({{rag_design}})
- [SQLite 索引与查询规划]({{sqlite_index}})

## 参考资料

- https://www.elastic.co/guide/en/elasticsearch/reference/current/rest-apis.html
- https://www.elastic.co/guide/en/elasticsearch/reference/current/retriever.html
- https://www.elastic.co/guide/en/elasticsearch/reference/current/search-profile.html
`),
		},
		{
			Key:       "vue_perf",
			Title:     "Vue 性能优化与交互设计",
			FolderKey: "frontend_folder",
			Tags:      []string{"vue", "frontend", "performance"},
			Markdown: strings.TrimSpace(`
# Vue 性能优化与交互设计

Vue 官方性能文档把优化分成两类：页面加载性能（首屏可见、可交互）和更新性能（交互后的响应速度）。这和 Notion 风格应用非常契合：侧边栏切页、搜索弹窗、卡片滑动都属于高频更新场景。如果我们只优化首屏包体，不处理更新路径，就会出现“页面打开还行，但用起来不丝滑”的问题。

对当前笔记系统最有效的策略有三条。第一，稳定 props 与局部更新：列表项只接收必要字段，避免父组件轻微变化导致整列重渲染。第二，按需加载和代码分割：非首屏能力（例如高级搜索、AI 面板）可延迟加载，减轻初始包。第三，控制大列表渲染成本：当笔记数量增加时，考虑虚拟列表或分段渲染。官方文档明确提到，大列表性能瓶颈通常来自 DOM 数量而非框架本身。

在交互设计上，建议把“感知性能”作为一等公民：点击侧边栏先本地渲染缓存，再后台刷新详情；搜索弹窗先展示最近访问，再异步补全匹配结果；长任务给出明确状态（如“思考中”“已保存”）。这些小策略能显著降低用户等待焦虑。对于编辑器，输入区和预览区分离是好的，但要限制全局样式副作用，避免一个 textarea 样式影响到 AI 面板等组件。

最后，性能优化必须可测量。可以用浏览器 Performance 面板、Vue DevTools 和 Web Vitals 指标观察变化：如果某次改动让交互延迟下降、渲染抖动减少，再把它固化进组件规范。不要追求“技巧列表”，要追求“可复现收益”。

## 关联页面

- [演示总览与页面跳转]({{demo_overview}})
- [RAG 系统设计与落地清单]({{rag_design}})

## 参考资料

- https://vuejs.org/guide/best-practices/performance
- https://cn.vuejs.org/guide/best-practices/performance
`),
		},
		{
			Key:       "demo_overview",
			Title:     "演示总览与页面跳转",
			FolderKey: "frontend_folder",
			Tags:      []string{"demo", "navigation"},
			Markdown: strings.TrimSpace(`
# 演示总览与页面跳转

这个页面用于答辩演示时的“讲解脚本”，建议按“问题 -> 方案 -> 验证”节奏进行。首先展示侧边栏结构：顶部可折叠、快速新建；中部是可嵌套页面树；底部是垃圾箱弹窗。然后进入主页，演示“最近访问”横向滑动与日期显示。接着打开搜索弹窗，输入关键词，演示分组结果与快速跳转。最后进入 AI 浮层，提问后展示可滚动问答记录。

为了让演示更连贯，推荐从基础工程主题切入，再过渡到 AI 主题。比如先打开 [Go GC 原理详解]({{go_gc}}) 说明系统内容深度，再跳转到 [RAG 系统设计与落地清单]({{rag_design}}) 展示检索增强能力，随后到 [Elasticsearch 检索建模实战]({{es_practice}}) 讲未来扩展路径。前端部分可以用 [Vue 性能优化与交互设计]({{vue_perf}}) 说明为什么我们做了“轻量列表 + 本地秒开 + 后台刷新”。

页面引用建议统一使用更接近 Notion 的双链格式，写成 [[页面标题]]。系统会在预览、保存和跳转时自动尝试解析为内部链接。这样做的价值是把“文档知识”变成可导航网络，而不是孤立文本；同时编辑区仍然保持人能读懂的标题，不会暴露内部 ID 地址。演示时可以现场点击链接，证明页面间引用真的可跳转，而不仅是静态展示。

如果要补充问答示例，建议提前准备三类问题：事实型（“某概念是什么”）、步骤型（“如何落地”）、比较型（“A 与 B 取舍”）。这能覆盖 RAG 的主要优势：引用证据、结构化回答、跨文档整合。答辩时别追求模型“无所不知”，重点展示系统工程能力与可持续扩展能力。

## 参考资料

- https://www.notion.com/help/markdown-and-keyboard-shortcuts
- https://developer.mozilla.org/docs/Web/API/HTML_Drag_and_Drop_API
`),
		},
	}

	folders = append(folders, expandedSeedFolders()...)
	pages = append(pages, expandedSeedPages()...)

	folderIDs := map[string]int64{}
	pageIDs := map[string]int64{}
	pageTitles := map[string]string{}
	for _, p := range pages {
		pageTitles[p.Key] = p.Title
	}

	for _, f := range folders {
		n, err := upsertByTitleAndParent(ctx, st, md, indexer, f.Title, nil, f.Tags, f.Markdown)
		if err != nil {
			log.Fatalf("upsert folder %s: %v", f.Title, err)
		}
		folderIDs[f.Key] = n.ID
		log.Printf("folder ready: %s (id=%d)", n.Title, n.ID)
	}

	for _, p := range pages {
		if n := contentLen(p.Markdown); n < 600 {
			log.Fatalf("seed page %q is too short: len=%d (<600)", p.Title, n)
		}
		parentID, ok := folderIDs[p.FolderKey]
		if !ok {
			log.Fatalf("folder key not found: %s", p.FolderKey)
		}
		parent := parentID
		n, err := upsertByTitleAndParent(ctx, st, md, indexer, p.Title, &parent, p.Tags, p.Markdown)
		if err != nil {
			log.Fatalf("upsert page %s: %v", p.Title, err)
		}
		pageIDs[p.Key] = n.ID
		log.Printf("page ready: %s (id=%d)", n.Title, n.ID)
	}

	for _, p := range pages {
		parentID := folderIDs[p.FolderKey]
		parent := parentID
		readable := applyReadableLinks(p.Markdown, pageTitles)
		htmlSource := applyNoteLinks(p.Markdown, pageIDs)
		n, err := upsertByTitleAndParent(ctx, st, md, indexer, p.Title, &parent, p.Tags, readable, htmlSource)
		if err != nil {
			log.Fatalf("resolve links for %s: %v", p.Title, err)
		}
		log.Printf("page linked: %s (id=%d)", n.Title, n.ID)
	}

	log.Printf("seed done: folders=%d pages=%d", len(folders), len(pages))
}

func newIndexer(st *store.Store) *rag.Service {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		log.Printf("OPENAI_API_KEY is empty, seed will skip RAG indexing")
		return nil
	}
	baseURL := getenv("OPENAI_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1")
	embedModel := getenv("OPENAI_EMBED_MODEL", "text-embedding-v3")
	chatModel := getenv("OPENAI_CHAT_MODEL", "qwen-plus")
	client := llm.NewOpenAICompatibleClient(&http.Client{Timeout: 120 * time.Second}, baseURL, apiKey, embedModel, chatModel)
	return rag.NewService(st, client, client, rag.Config{
		MaxChunkChars: 900,
		TopK:          5,
	})
}

func upsertByTitleAndParent(
	ctx context.Context,
	st *store.Store,
	md goldmark.Markdown,
	indexer *rag.Service,
	title string,
	parentID *int64,
	tags []string,
	markdown string,
	renderSource ...string,
) (store.Note, error) {
	note, found, err := findByTitleAndParent(ctx, st, title, parentID)
	if err != nil {
		return store.Note{}, err
	}
	source := markdown
	if len(renderSource) > 0 && strings.TrimSpace(renderSource[0]) != "" {
		source = renderSource[0]
	}
	html, err := renderMarkdown(md, source)
	if err != nil {
		return store.Note{}, err
	}

	if found {
		updated, err := st.UpdateNote(ctx, note.ID, store.NoteInput{
			ParentID: parentID,
			Title:    title,
			Markdown: markdown,
			HTML:     html,
			Tags:     tags,
		})
		if err != nil {
			return store.Note{}, err
		}
		if updated.IsArchived {
			updated, err = st.SetArchived(ctx, updated.ID, false)
			if err != nil {
				return store.Note{}, err
			}
		}
		if indexer != nil {
			if err := indexer.IndexNote(ctx, updated.ID, updated.Markdown); err != nil {
				log.Printf("index warning for note %d: %v", updated.ID, err)
			}
		}
		return updated, nil
	}

	created, err := st.CreateNote(ctx, store.NoteInput{
		ParentID: parentID,
		Title:    title,
		Markdown: markdown,
		HTML:     html,
		Tags:     tags,
	})
	if err != nil {
		return store.Note{}, err
	}
	if indexer != nil {
		if err := indexer.IndexNote(ctx, created.ID, created.Markdown); err != nil {
			log.Printf("index warning for note %d: %v", created.ID, err)
		}
	}
	return created, nil
}

func findByTitleAndParent(ctx context.Context, st *store.Store, title string, parentID *int64) (store.Note, bool, error) {
	all, err := st.ListNotes(ctx, store.NoteFilter{IncludeArchived: true})
	if err != nil {
		return store.Note{}, false, err
	}
	target := strings.TrimSpace(strings.ToLower(title))
	for _, n := range all {
		if strings.TrimSpace(strings.ToLower(n.Title)) != target {
			continue
		}
		if sameParent(n.ParentID, parentID) {
			return n, true, nil
		}
	}
	return store.Note{}, false, nil
}

func sameParent(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func renderMarkdown(md goldmark.Markdown, markdown string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func applyReadableLinks(markdown string, titles map[string]string) string {
	out := markdown
	for key, title := range titles {
		token := "{{" + key + "}}"
		linkRE := regexp.MustCompile(`\[[^\]]+\]\(` + regexp.QuoteMeta(token) + `\)`)
		out = linkRE.ReplaceAllString(out, "[["+title+"]]")
		out = strings.ReplaceAll(out, token, "[["+title+"]]")
	}
	return out
}

func applyNoteLinks(markdown string, ids map[string]int64) string {
	out := markdown
	for key, id := range ids {
		token := "{{" + key + "}}"
		out = strings.ReplaceAll(out, token, fmt.Sprintf("note://%d", id))
	}
	return out
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func contentLen(s string) int {
	n := 0
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || r == ' ' {
			continue
		}
		n++
	}
	return n
}

type expandedSeedSpec struct {
	Key       string
	Title     string
	FolderKey string
	Tags      []string
	Field     string
	Question  string
	Scene     string
	Method    string
	Practice  []string
	Risk      string
	Example   string
	Refs      []string
}

func expandedSeedFolders() []seedDoc {
	return []seedDoc{
		{
			Key:      "data_folder",
			Title:    "数据存储",
			Tags:     []string{"folder", "catalog", "database"},
			Markdown: "# 数据存储\n\n这个目录收纳事务、仓库分层、审计、数据治理与检索基础设施页面。",
		},
		{
			Key:      "product_folder",
			Title:    "产品与规划",
			Tags:     []string{"folder", "catalog", "product"},
			Markdown: "# 产品与规划\n\n这个目录收纳路线图、指标、用户研究、推荐与商业化设计页面。",
		},
		{
			Key:      "society_folder",
			Title:    "人文社会",
			Tags:     []string{"folder", "catalog", "society"},
			Markdown: "# 人文社会\n\n这个目录收纳教育、心理、历史、公共政策、媒体素养与法律风险页面。",
		},
		{
			Key:      "science_folder",
			Title:    "科学与健康",
			Tags:     []string{"folder", "catalog", "science"},
			Markdown: "# 科学与健康\n\n这个目录收纳睡眠、营养、生物信息、能源、农业与环境适应页面。",
		},
		{
			Key:      "ops_folder",
			Title:    "安全与运维",
			Tags:     []string{"folder", "catalog", "ops"},
			Markdown: "# 安全与运维\n\n这个目录收纳威胁建模、事故响应、可观测性、发布工程与供应链韧性页面。",
		},
		{
			Key:      "creation_folder",
			Title:    "设计与创作",
			Tags:     []string{"folder", "catalog", "design"},
			Markdown: "# 设计与创作\n\n这个目录收纳设计系统、信息架构、研究写作与界面表达页面。",
		},
	}
}

func expandedSeedPages() []seedDoc {
	titles := map[string]string{}
	specs := expandedSeedSpecs()
	for _, s := range specs {
		titles[s.Key] = s.Title
	}
	out := make([]seedDoc, 0, len(specs))
	for _, s := range specs {
		out = append(out, seedDoc{
			Key:       s.Key,
			Title:     s.Title,
			FolderKey: s.FolderKey,
			Tags:      s.Tags,
			Markdown:  expandedMarkdown(s, titles),
		})
	}
	return out
}

func expandedMarkdown(s expandedSeedSpec, titles map[string]string) string {
	var b strings.Builder
	b.WriteString("# " + s.Title + "\n\n")
	b.WriteString("## 场景观察\n\n")
	b.WriteString(s.Scene + " 在实际工作中，一个主题常常同时牵涉技术、组织和人的选择：数据如何被记录，界面如何提示用户，指标如何反映真实目标，风险如何被提前暴露。把这些层面写在同一篇笔记里，可以让它和其他页面形成网络。例如本页会连接到" + refSentence(s.Refs, titles) + "，读者可以从不同入口重新进入同一个问题。\n\n")
	b.WriteString("## 方法框架\n\n")
	b.WriteString(s.Method + " 一个可靠的方法通常包含三步：先把对象边界写清楚，再定义判断标准，最后保留复盘证据。边界让团队知道什么不做，标准让讨论不漂移，证据让以后回看时能理解当时为什么那样决定。这个框架也适合笔记系统本身：每一篇长文既是一段内容，也是一组可被引用、检索和重组的知识片段。\n\n")
	b.WriteString("## 实践清单\n\n")
	for _, p := range s.Practice {
		b.WriteString("- " + p + "\n")
	}
	b.WriteString("\n")
	b.WriteString("## 风险与边界\n\n")
	b.WriteString(s.Risk + " 因此，记录时要刻意写出假设条件、反例和不适用场景。没有边界的笔记很容易在复用时被误解；有边界的笔记虽然看起来更克制，却能让读者知道什么时候应该迁移经验，什么时候应该重新验证。\n\n")
	b.WriteString("## 示例\n\n")
	b.WriteString(s.Example + " 这个例子可以在后续研究、方案评审或学习复盘时继续扩展：如果它被证明有效，就沉淀为模板；如果出现偏差，就把偏差写回笔记，形成下一轮改进的证据链。\n\n")
	b.WriteString("## 关联页面\n\n")
	for _, key := range s.Refs {
		title := titles[key]
		if title == "" {
			title = key
		}
		b.WriteString("- [" + title + "]({{" + key + "}})\n")
	}
	return strings.TrimSpace(b.String())
}

func refSentence(refs []string, titles map[string]string) string {
	names := make([]string, 0, len(refs))
	for _, key := range refs {
		if title := titles[key]; title != "" {
			names = append(names, "《"+title+"》")
		}
	}
	if len(names) == 0 {
		return "相关页面"
	}
	return strings.Join(names, "、")
}

func expandedSeedSpecs() []expandedSeedSpec {
	return []expandedSeedSpec{
		{
			Key:       "go_learning_updated",
			Title:     "Go 学习笔记（已更新）",
			FolderKey: "backend_folder",
			Tags:      []string{"go", "learning", "backend"},
			Field:     "后端工程",
			Question:  "如何把零散的 Go 语法学习整理成可复用的工程能力",
			Scene:     "初学 Go 时很容易按语法点记录：切片、map、接口、goroutine、context、错误处理各写一点。问题是这些知识如果不放进真实任务，就会停留在记忆层面。后端服务需要把语言特性连接到 API、数据库、并发和测试。",
			Method:    "建议用项目任务重组学习笔记：一次请求如何进入 handler，如何调用 store，如何处理错误，如何写测试，如何在并发任务里传播取消信号。每个语法点都配一个系统场景，读者才能知道它为什么重要。",
			Practice:  []string{"把语法笔记改写成问题，例如“什么时候用指针接收者”。", "每周选择一段项目代码复盘，记录可改进的命名、错误和测试。", "将 go test、race、pprof 和日志观察纳入学习路径。"},
			Risk:      "只追求知识点覆盖会让笔记越来越散，真正写服务时仍然不知道从哪里下手。",
			Example:   "学习 context 时，不只记录 API，还要观察 HTTP 超时、数据库查询取消、RAG 索引任务停止和 goroutine 退出之间的关系。",
			Refs:      []string{"go_pipeline", "go_error_logging", "server_test_strategy"},
		},
		{
			Key:       "go_error_logging",
			Title:     "Go 错误处理与日志实践",
			FolderKey: "backend_folder",
			Tags:      []string{"go", "backend", "logging"},
			Field:     "后端工程",
			Question:  "怎样让错误既能被用户理解，又能被工程团队定位",
			Scene:     "Go 服务里最常见的问题不是没有错误返回，而是错误被层层包装后丢失语义，日志又只留下字符串。接口层需要给用户稳定响应，业务层需要保留领域原因，基础设施层需要记录依赖、耗时和重试信息。",
			Method:    "建议把错误分成可预期业务错误、依赖错误和程序缺陷三类。业务错误返回明确 code，依赖错误记录 request id 和下游响应，程序缺陷进入告警。日志字段保持结构化，至少包含 trace、用户动作、资源 id、耗时和错误类别。",
			Practice:  []string{"在 handler 边界统一翻译错误，避免每个接口手写响应。", "日志用键值结构，不把所有上下文拼成一段长文本。", "对高频错误设置采样，对高危错误保留完整上下文。"},
			Risk:      "过度包装会让调用栈和领域含义同时变得模糊，过度记录又可能泄露隐私或让日志成本失控。",
			Example:   "例如保存笔记失败时，用户只需要看到“保存失败，请稍后重试”，而服务端日志要区分是 Markdown 渲染失败、SQLite 忙等待超时，还是 RAG 索引阶段的 embedding 调用失败。",
			Refs:      []string{"http_api", "observability_slo", "incident_response"},
		},
		{
			Key:       "server_test_strategy",
			Title:     "服务端测试策略清单",
			FolderKey: "backend_folder",
			Tags:      []string{"backend", "test", "api"},
			Field:     "后端工程",
			Question:  "如何用有限测试覆盖最容易回归的服务端行为",
			Scene:     "笔记系统同时有 CRUD、归档、标签、搜索、AI 问答和导入导出。若只写快乐路径测试，改一个字段就可能破坏搜索过滤、父子页面或内部链接解析。测试策略必须围绕业务风险，而不是围绕文件数量平均分配。",
			Method:    "可以把测试分为三层：存储层验证约束和迁移，API 层验证状态码与响应结构，端到端脚本验证核心用户旅程。对 RAG 这类依赖模型的能力，应使用假客户端固定输出，把不稳定性隔离在少量集成测试里。",
			Practice:  []string{"每个状态迁移至少有成功和非法状态两个用例。", "搜索、标签、归档同时出现时要测组合过滤。", "新增字段时补一条旧数据库迁移测试，避免真实用户升级失败。"},
			Risk:      "测试过少会放大回归，测试过细则会把实现细节冻住，让重构变得困难。",
			Example:   "例如删除页面时，不仅要断言 notes 表记录消失，还要确认标签、分块、复习题和块内容是否按外键策略一起清理，避免界面看似删除成功，检索结果却仍然返回旧片段。",
			Refs:      []string{"go_error_logging", "sqlite_index", "rag_eval_recall"},
		},
		{
			Key:       "rag_eval_recall",
			Title:     "RAG 评估指标与召回优化",
			FolderKey: "ai_folder",
			Tags:      []string{"rag", "llm", "evaluation"},
			Field:     "AI 与检索",
			Question:  "怎样判断 RAG 不是“看起来能答”，而是真的检索到了正确证据",
			Scene:     "RAG 失败常被误判为大模型能力差，实际根因可能是切块太碎、标题权重不足、标签过滤过严或召回结果没有重排。没有评估集时，团队只能凭几次演示体验判断质量，很容易在数据增长后失控。",
			Method:    "评估应拆成检索和生成两段。检索看 recall@k、MRR、命中来源覆盖率，生成看答案正确性、引用一致性和拒答合理性。先固定一组事实型、步骤型、跨文档综合型问题，再记录每次答案用到哪些 chunk。",
			Practice:  []string{"为每个问题维护期望命中文档，而不是只保存标准答案。", "同时评估关键词检索、向量检索和混合检索。", "把低分案例回写成新的切块、标签或标题改进任务。"},
			Risk:      "只看最终回答会掩盖召回错误，模型可能凭常识给出漂亮但无来源的答案。",
			Example:   "如果用户问“如何处理归档页面的搜索结果”，正确证据可能来自 HTTP API、SQLite 查询和前端交互三处；评估时应检查这些页面是否进入 top-k，而不是只看回答是否像样。",
			Refs:      []string{"rag_design", "vector_prompt_notes", "sqlite_index"},
		},
		{
			Key:       "vector_prompt_notes",
			Title:     "向量检索 Prompt 设计备忘",
			FolderKey: "ai_folder",
			Tags:      []string{"rag", "llm", "prompt"},
			Field:     "AI 与检索",
			Question:  "检索上下文进入提示词后，如何减少遗漏、误引和幻觉",
			Scene:     "向量检索只负责把相关片段找出来，最后是否能变成可靠答案，还取决于提示词如何组织证据。若上下文没有编号、没有来源标题、没有拒答规则，模型会把多个片段混在一起，甚至补充未被检索到的事实。",
			Method:    "提示词应把任务、证据、输出格式和不确定处理分开。证据片段带上 note id、标题和 chunk 序号，答案中尽量引用来源。若证据不足，模型应说明缺口，并提出下一步检索词，而不是编造完整结论。",
			Practice:  []string{"把用户问题原文和改写后的检索查询都保留下来，便于复盘。", "对跨文档问题要求先列证据再给结论。", "限制回答只使用检索片段中出现的信息，必要时显式拒答。"},
			Risk:      "提示词越长越容易吞掉关键信息，过度模板化又会让回答僵硬。",
			Example:   "答复“深色模式为何影响可用性”时，应先引用界面规范，再引用可用性走查，而不是直接给一段通用设计原则。这样用户能顺着链接继续读原文。",
			Refs:      []string{"rag_eval_recall", "dark_mode_spec", "usability_walkthrough"},
		},
		{
			Key:       "usability_walkthrough",
			Title:     "可用性走查清单",
			FolderKey: "frontend_folder",
			Tags:      []string{"frontend", "ux", "checklist"},
			Field:     "前端体验",
			Question:  "怎样用走查发现用户在真实操作中会卡住的地方",
			Scene:     "一个笔记应用看起来简洁，不代表它真的好用。用户会在新建、搜索、编辑、保存、归档、恢复和 AI 问答之间来回切换；任何一个动作反馈不清，都可能让人误以为内容丢失或系统无响应。",
			Method:    "可用性走查可以围绕任务流展开：从第一次打开系统开始，记录每一步是否有明确入口、当前状态、可撤销路径和错误提示。走查人员不要只看静态页面，要带着具体任务操作，例如“找回昨天归档的 RAG 笔记”。",
			Practice:  []string{"每个关键操作都要有即时反馈，如保存中、已保存、失败原因。", "空状态给出下一步入口，而不是只显示空白。", "危险操作提供恢复路径或二次确认，并在垃圾箱中可追踪。"},
			Risk:      "走查若只由开发者完成，很容易忽略新用户对术语、图标和层级的陌生感。",
			Example:   "搜索弹窗中若同时支持标题、正文、标签和最近访问，应让命中来源可见；否则用户无法判断为什么某条结果排在前面，也不利于后续改进搜索排序。",
			Refs:      []string{"vue_perf", "dark_mode_spec", "information_architecture"},
		},
		{
			Key:       "dark_mode_spec",
			Title:     "深色模式界面规范",
			FolderKey: "frontend_folder",
			Tags:      []string{"frontend", "design", "dark-mode"},
			Field:     "前端体验",
			Question:  "深色界面怎样保持层级清楚而不显得灰、脏、累眼",
			Scene:     "深色模式不是把白底反转成黑底。笔记系统里有侧边栏、编辑器、预览、弹窗、标签、按钮和 AI 面板，若所有区域都使用相近的深蓝灰，用户会难以判断层级，长时间写作也容易疲劳。",
			Method:    "规范应定义背景层级、边框透明度、文字对比、状态色和焦点样式。主背景负责安静，交互面负责可发现，危险和成功状态只在必要位置出现。代码层面则把颜色沉淀为变量，避免组件各写各的。",
			Practice:  []string{"正文阅读区域优先保证对比度和行高，不追求炫目效果。", "按钮状态至少区分默认、悬停、激活、禁用和危险。", "标签颜色保持克制，避免一屏出现过多高饱和色块。"},
			Risk:      "过暗会降低可读性，过亮又会破坏夜间使用的舒适感。",
			Example:   "AI 浮层可使用稍高一层背景，消息气泡通过边框和轻微底色区分角色，而不是用大面积亮色抢走编辑器注意力。",
			Refs:      []string{"usability_walkthrough", "design_system_tokens", "vue_perf"},
		},
		{
			Key:       "knowledge_base_roadmap",
			Title:     "个人知识库路线图",
			FolderKey: "product_folder",
			Tags:      []string{"product", "roadmap", "knowledge-base"},
			Field:     "产品与规划",
			Question:  "个人知识库从记录工具演进到学习工作台，应先做哪些能力",
			Scene:     "很多知识库产品一开始只解决记录和分类，用户笔记增加后才暴露真正痛点：找不到、串不起来、复习不了、无法生成阶段性成果。路线图需要围绕学习闭环，而不是围绕功能清单堆叠。",
			Method:    "建议按四个阶段推进：先保证稳定记录和导出，再做好搜索与内部链接，然后加入智能洞察、复习卡片和推荐，最后扩展周报、研究室和跨文件导入。每个阶段都要有可度量指标，例如活跃笔记数、搜索成功率、复习完成率。",
			Practice:  []string{"优先打磨高频路径：创建、编辑、搜索、跳转和恢复。", "把 AI 能力绑定到明确任务，而不是只放一个聊天框。", "路线图每轮只承诺少量关键结果，保留探索空间。"},
			Risk:      "过早追求复杂协作或花哨图谱，会稀释个人用户最核心的记录和复用体验。",
			Example:   "当系统已有足够长文和内链后，内容推荐不再只是随机相关，而可以基于标题、标签、链接和向量相似度给出“下一篇该读什么”。",
			Refs:      []string{"recommendation_design", "product_metrics_north_star", "rag_eval_recall"},
		},
		{
			Key:       "recommendation_design",
			Title:     "内容推荐功能设计",
			FolderKey: "product_folder",
			Tags:      []string{"product", "recommendation", "ai"},
			Field:     "产品与规划",
			Question:  "笔记系统里的推荐应服务学习，而不是制造信息流噪声",
			Scene:     "推荐功能很容易被做成“相似内容列表”，但个人知识库的目标不是延长停留时间，而是帮助用户建立连接、发现遗漏和推进下一步行动。推荐理由必须透明，否则用户很难信任系统判断。",
			Method:    "推荐可以混合四类信号：标签相同、标题或正文相似、显式内链、最近任务相关。结果展示时给出原因，例如“同属 RAG 评估”“被当前页面引用”“本周多次编辑”。用户点击后的行为再反哺排序。",
			Practice:  []string{"推荐列表数量保持少而准，避免占据编辑主界面。", "每条推荐显示理由和相关强度，方便用户判断。", "允许用户忽略或收藏推荐，把反馈写入后续排序。"},
			Risk:      "如果推荐只追求相似，系统会不断把用户困在同一主题里，削弱跨领域联想。",
			Example:   "在阅读“睡眠与认知表现”时，系统可推荐“学习科学中的间隔复习”，理由不是关键词相同，而是二者都讨论记忆巩固和复盘节奏。",
			Refs:      []string{"knowledge_base_roadmap", "learning_science_review", "vector_prompt_notes"},
		},
		{
			Key:       "postgres_transaction_isolation",
			Title:     "PostgreSQL 事务隔离与并发现象",
			FolderKey: "data_folder",
			Tags:      []string{"database", "postgres", "transaction"},
			Field:     "数据存储",
			Question:  "事务隔离级别如何影响并发读写的正确性和性能",
			Scene:     "业务系统常把“用了事务”误解为“不会出错”，但脏读、不可重复读、幻读和写偏斜属于不同层次的问题。笔记系统虽然以 SQLite 为主，理解隔离级别仍有价值，因为未来团队协作、同步和导入任务都会遇到并发写入。",
			Method:    "分析事务时先写出不变量，例如同一页面不能同时存在两个活跃版本、同一标签不能重复绑定。再判断默认隔离级别是否能保护该不变量，必要时使用唯一约束、显式锁、乐观版本号或串行化事务。",
			Practice:  []string{"不要只依赖应用层判断唯一性，关键约束应落到数据库。", "长事务中避免等待用户输入，减少锁持有时间。", "对重试安全的操作设计幂等键，避免网络抖动造成重复写。"},
			Risk:      "隔离级别越高不一定越好，它可能带来更多阻塞、死锁和重试成本。",
			Example:   "如果两个客户端同时给同一篇笔记新增同名标签，应用层先查后插可能发生竞争；唯一约束能把问题变成可捕获的冲突，再由 API 返回稳定错误。",
			Refs:      []string{"sqlite_index", "event_sourcing_audit", "server_test_strategy"},
		},
		{
			Key:       "data_warehouse_layering",
			Title:     "数据仓库分层与指标口径",
			FolderKey: "data_folder",
			Tags:      []string{"data", "warehouse", "metrics"},
			Field:     "数据存储",
			Question:  "为什么同一个指标在不同报表里经常算不一致",
			Scene:     "当产品、运营和研发各自从原始表取数时，“活跃用户”“有效笔记”“AI 使用次数”会出现多个口径。短期看每个人都很快，长期看会议会被口径争论占满，决策也失去可信基础。",
			Method:    "数据仓库分层的价值在于把原始事件、清洗明细、主题模型和应用指标拆开。ODS 保留事实，DWD 做清洗，DWS 沉淀主题汇总，ADS 面向看板和业务问题。每层都写清楚更新频率、字段含义和质量校验。",
			Practice:  []string{"核心指标必须有口径文档和负责人。", "埋点变更要记录版本，避免新旧事件混算。", "看板展示趋势时同时给出异常标注和数据延迟说明。"},
			Risk:      "过早建设复杂数仓会增加维护成本，但完全没有分层会让指标在增长后迅速失控。",
			Example:   "“本周有效学习笔记”可以定义为本周更新、正文超过一定长度、且至少包含一个标签或内部链接的页面，这比单纯统计新建数量更能反映学习质量。",
			Refs:      []string{"product_metrics_north_star", "privacy_data_governance", "knowledge_base_roadmap"},
		},
		{
			Key:       "event_sourcing_audit",
			Title:     "事件溯源与审计日志",
			FolderKey: "data_folder",
			Tags:      []string{"architecture", "audit", "event"},
			Field:     "数据存储",
			Question:  "什么时候应该记录事件，而不只是保存最新状态",
			Scene:     "普通 CRUD 表擅长展示当前状态，却不擅长解释状态如何变化。对笔记系统来说，归档、恢复、复制、导入、AI 优化和删除都可能需要追踪来源：是谁触发、何时触发、影响了哪些资源、能否回滚。",
			Method:    "事件溯源不一定要全量重构系统，可以先对关键动作建立审计表。事件包含 actor、action、target、payload、request id 和时间。当前状态仍由主表承载，审计事件用于追责、恢复、分析和问题定位。",
			Practice:  []string{"先记录不可逆或高风险动作，如永久删除、批量导入和自动清理。", "事件 payload 避免保存敏感正文，可保存摘要和资源 id。", "后台任务也要有 actor 标识，区分用户动作和系统动作。"},
			Risk:      "事件表若没有查询场景，很快会成为没人维护的大日志。",
			Example:   "当用户反馈一篇页面不见了，系统可以从审计事件看到它先被归档，再被自动清理任务删除，并定位触发清理的策略版本。",
			Refs:      []string{"postgres_transaction_isolation", "incident_response", "contract_risk"},
		},
		{
			Key:       "privacy_data_governance",
			Title:     "隐私数据治理备忘录",
			FolderKey: "data_folder",
			Tags:      []string{"privacy", "governance", "security"},
			Field:     "数据存储",
			Question:  "个人知识库如何在智能化能力和隐私保护之间取得平衡",
			Scene:     "笔记里可能包含学习计划、账号线索、商业想法、健康记录和私人反思。接入 AI、推荐和向量检索后，系统会复制、切块、索引这些内容，数据流比传统编辑器复杂得多。",
			Method:    "治理应先画数据流：哪些内容只在本地保存，哪些会发给模型，哪些会进入日志、向量库或导出文件。再定义最小化原则、保留周期、删除策略和用户可见说明。默认不要把敏感正文写进普通日志。",
			Practice:  []string{"对 AI 调用展示清晰开关和提供方说明。", "删除笔记时同步清理分块、向量和推荐缓存。", "对导入文件保留来源信息，方便用户以后追踪和撤回。"},
			Risk:      "隐私治理若只写在文档里，而没有落实到数据结构和接口行为，很难在真实事故中发挥作用。",
			Example:   "用户关闭云端模型后，RAG 问答可以降级为本地搜索摘要，而不是悄悄继续发送正文到外部服务。",
			Refs:      []string{"security_threat_modeling", "event_sourcing_audit", "vector_prompt_notes"},
		},
		{
			Key:       "product_metrics_north_star",
			Title:     "北极星指标与学习产品增长",
			FolderKey: "product_folder",
			Tags:      []string{"product", "metrics", "growth"},
			Field:     "产品与规划",
			Question:  "学习类工具的增长指标如何避免鼓励无效使用",
			Scene:     "很多产品会用日活、打开次数或停留时长衡量增长，但学习工具的目标不是让用户一直刷界面，而是帮助他们记录、理解、复习和产出。错误指标会诱导团队做更多提醒、弹窗和内容流，却不一定提升学习效果。",
			Method:    "北极星指标应贴近用户获得的真实价值。对个人知识库，可以考虑“每周被复用的有效笔记数”或“完成复盘的主题数”。它们要求用户不仅创建内容，还能通过搜索、链接、问答或周报重新使用内容。",
			Practice:  []string{"指标设计同时包含数量和质量门槛，避免刷空笔记。", "把输入指标、过程指标和结果指标拆开观察。", "每个增长实验都写清楚可能伤害的体验指标。"},
			Risk:      "指标一旦成为目标，就可能被团队和用户行为反向塑造。",
			Example:   "如果只看 AI 提问次数，系统可能鼓励用户频繁追问；若看“回答后打开来源笔记并继续编辑”的比例，更接近学习闭环。",
			Refs:      []string{"data_warehouse_layering", "user_research_interview", "recommendation_design"},
		},
		{
			Key:       "roadmap_prioritization",
			Title:     "产品路线图优先级方法",
			FolderKey: "product_folder",
			Tags:      []string{"product", "roadmap", "strategy"},
			Field:     "产品与规划",
			Question:  "需求很多时，如何决定下一轮真正应该做什么",
			Scene:     "笔记系统可以扩展的方向非常多：协作、移动端、OCR、知识图谱、复习、研究室、模板、语音输入。若没有优先级方法，路线图会被最新反馈牵着走，团队持续切换上下文。",
			Method:    "可以结合 RICE、机会解决树和风险拆解。先判断需求影响的是获客、激活、留存还是学习成果，再估算触达人数、收益、信心和成本。对高不确定需求先做原型或手动流程验证，而不是直接完整开发。",
			Practice:  []string{"每个需求写清目标用户、触发场景和成功指标。", "把基础体验债务和新功能放在同一张优先级表里比较。", "保留一部分容量处理技术债和稳定性问题。"},
			Risk:      "过度量化会制造精确幻觉，尤其在样本少、反馈偏差大的早期项目中。",
			Example:   "若搜索成功率低，继续做高级图谱可能不如先改搜索排序和标签管理，因为后者直接影响用户每天能否找回内容。",
			Refs:      []string{"knowledge_base_roadmap", "product_metrics_north_star", "usability_walkthrough"},
		},
		{
			Key:       "user_research_interview",
			Title:     "用户访谈与需求验证",
			FolderKey: "product_folder",
			Tags:      []string{"product", "research", "ux"},
			Field:     "产品与规划",
			Question:  "怎样从用户叙述中识别真实需求，而不被解决方案诱导",
			Scene:     "用户常会直接提出功能请求，例如“我要一个知识图谱”或“帮我自动总结所有笔记”。访谈的任务不是立刻答应，而是追问他们在什么场景下遇到什么阻碍、现在如何绕过、失败成本是什么。",
			Method:    "访谈问题尽量围绕过去行为，而不是未来想象。询问最近一次记录、搜索、复习或写周报的过程，要求用户展示真实材料。记录时区分事实、情绪、解释和机会点，访谈后再归纳模式。",
			Practice:  []string{"避免问“你会不会用这个功能”，改问“上次你怎么解决”。", "把访谈对象按学习阶段、内容类型和使用频率分组。", "每次访谈结束后提炼一条可验证假设。"},
			Risk:      "少量高表达用户的意见可能非常有感染力，却不代表多数人的真实路径。",
			Example:   "如果三位用户都说“找不到旧笔记”，下一步不一定是做复杂图谱，可能是改善标题建议、最近访问、标签筛选和搜索结果摘要。",
			Refs:      []string{"roadmap_prioritization", "information_architecture", "product_metrics_north_star"},
		},
		{
			Key:       "pricing_packaging",
			Title:     "SaaS 定价与套餐设计",
			FolderKey: "product_folder",
			Tags:      []string{"product", "pricing", "saas"},
			Field:     "产品与规划",
			Question:  "如何让价格反映价值，同时不破坏用户信任",
			Scene:     "知识工具若未来商业化，定价不只是给功能贴价格。用户会关心数据可迁移性、AI 成本、隐私边界、同步能力和长期可用性。套餐设计不清，会让用户担心内容被锁住。",
			Method:    "定价可以围绕价值阶梯：免费层保证记录和导出，个人高级层提供 AI、同步和高级检索，团队层提供权限、审计和协作。每一层都要说明限制来自成本、风险还是组织管理，而不是人为制造痛点。",
			Practice:  []string{"导出和基础访问不应成为强迫付费的杠杆。", "AI 额度用透明用量展示，避免用户不知道成本如何产生。", "团队功能按管理价值收费，如权限、审计、模板和共享空间。"},
			Risk:      "过早商业化会影响产品判断，过晚验证支付意愿又可能让路线图偏离可持续经营。",
			Example:   "RAG 问答可以限制免费额度，但仍允许用户使用本地搜索和 Markdown 导出，这样既控制模型成本，也保护知识资产的可迁移性。",
			Refs:      []string{"privacy_data_governance", "product_metrics_north_star", "contract_risk"},
		},
		{
			Key:       "learning_science_review",
			Title:     "学习科学中的间隔复习",
			FolderKey: "society_folder",
			Tags:      []string{"education", "learning", "review"},
			Field:     "人文社会",
			Question:  "为什么重复阅读不等于真正掌握，复习间隔如何帮助长期记忆",
			Scene:     "很多人写了大量笔记，却很少回看；临近考试或项目汇报时再集中阅读，感觉熟悉但迁移能力不强。学习科学提醒我们，提取练习、间隔复习和交错练习比单纯重读更能巩固记忆。",
			Method:    "在知识库中，可以把笔记转化为复习问题、卡片和应用任务。第一次复习检查概念，第二次复习要求举例，第三次复习尝试跨主题连接。间隔长度根据熟悉程度调整，而不是机械每天重复。",
			Practice:  []string{"每篇长文至少沉淀三个可回答问题。", "复习时先闭卷回忆，再打开原文核对。", "把答错原因写回笔记，而不是只标记“不会”。"},
			Risk:      "复习系统若只追求打卡，会把学习变成机械任务，削弱理解和迁移。",
			Example:   "读完 RAG 评估后，可以生成问题：“为什么只看最终回答不够？”如果回答时能联系召回、重排和引用一致性，说明理解已经跨过单点记忆。",
			Refs:      []string{"recommendation_design", "health_sleep_cognition", "writing_research_notes"},
		},
		{
			Key:       "cognitive_bias_decision",
			Title:     "认知偏差与决策记录",
			FolderKey: "society_folder",
			Tags:      []string{"psychology", "decision", "thinking"},
			Field:     "人文社会",
			Question:  "如何用记录减少事后合理化和过度自信",
			Scene:     "人在做技术选型、产品优先级或投资判断时，常低估不确定性。结果好时觉得自己早有预见，结果差时又容易把原因推给外部环境。决策记录能把当时的证据、假设和反对意见固定下来。",
			Method:    "每次重要决策前写下问题、可选方案、预期结果、关键风险、触发复盘的时间点。复盘时不要只问结果对错，还要看当时推理是否充分、是否忽视了反例、是否被沉没成本牵制。",
			Practice:  []string{"重要选择至少写一个反方版本，强迫自己看到替代解释。", "把信心程度量化为区间，而不是只写“很有把握”。", "复盘时区分运气、执行质量和判断质量。"},
			Risk:      "记录若变成证明自己正确的材料，就会强化偏差而不是纠正偏差。",
			Example:   "决定是否引入 Elasticsearch 时，可以记录当前数据规模、查询延迟、运维成本和替代方案；三个月后再看真实增长是否达到预期。",
			Refs:      []string{"roadmap_prioritization", "behavioral_finance_notes", "es_practice"},
		},
		{
			Key:       "urban_mobility_planning",
			Title:     "城市交通与慢行系统规划",
			FolderKey: "society_folder",
			Tags:      []string{"urban", "mobility", "planning"},
			Field:     "人文社会",
			Question:  "城市交通优化为什么不能只看机动车通行效率",
			Scene:     "交通系统服务的是人的到达，而不只是车辆速度。若道路设计只追求机动车通行，步行、自行车、公共交通换乘和街道安全都会被挤压。结果可能是路更宽了，生活半径却更不友好。",
			Method:    "慢行系统规划应关注连续性、安全性、舒适性和可达性。连续性要求人行道和骑行道不频繁中断，安全性要求路口速度可控，舒适性涉及遮阴、照明和噪声，可达性则看学校、社区、公交站之间的连接。",
			Practice:  []string{"用十五分钟生活圈检查日常服务是否可步行到达。", "路口优先保护弱势交通参与者，而不是只优化车流相位。", "把事故、热岛、商业活力和居民反馈一起纳入评估。"},
			Risk:      "只用平均车速衡量交通改善，会掩盖老人、儿童和非机动车用户的成本。",
			Example:   "一个学校周边改造方案，可以先降低路口转弯半径、增加安全岛和接送缓冲区，再评估学生独立通学比例是否提升。",
			Refs:      []string{"climate_adaptation_city", "public_policy_evidence", "user_research_interview"},
		},
		{
			Key:       "public_policy_evidence",
			Title:     "公共政策中的证据评估",
			FolderKey: "society_folder",
			Tags:      []string{"policy", "evidence", "governance"},
			Field:     "人文社会",
			Question:  "政策效果如何从主观印象转向可检验判断",
			Scene:     "公共政策经常面对多目标冲突：效率、公平、成本、接受度和长期影响不可能同时最优。若只听个别案例或短期舆论，政策很容易在压力下频繁摇摆。",
			Method:    "证据评估需要先定义目标和受影响群体，再选择合适指标。能随机试验时做试点，不能随机时用准实验、前后对比或相似地区对照。定量结果之外，还要保留访谈和一线执行反馈。",
			Practice:  []string{"政策上线前写清楚预期影响和可能副作用。", "对弱势群体单独观察，避免平均数掩盖分布差异。", "每次复盘同时讨论数据质量和执行偏差。"},
			Risk:      "证据不是消灭价值判断，而是让价值冲突更透明。",
			Example:   "评估城市慢行改造时，不能只看拥堵指数，还要看交通伤害、商铺客流、居民满意度和不同年龄群体的出行自由度。",
			Refs:      []string{"urban_mobility_planning", "data_warehouse_layering", "contract_risk"},
		},
		{
			Key:       "contract_risk",
			Title:     "合同条款中的风险识别",
			FolderKey: "society_folder",
			Tags:      []string{"law", "contract", "risk"},
			Field:     "人文社会",
			Question:  "非法律专业人员阅读合同时应优先关注哪些风险",
			Scene:     "合同不是形式文件，而是未来发生争议时的行动说明。很多风险藏在交付标准、验收期限、责任限制、数据归属、终止条款和争议解决中。只看价格和标题，很容易忽略真正影响合作的边界。",
			Method:    "阅读合同时先标出角色、义务、时间、成果物、违约后果和退出路径。对模糊表达追问可验证标准，例如“及时”“合理”“高质量”都需要更具体的判断依据。涉及隐私、知识产权和长期绑定时，应寻求专业意见。",
			Practice:  []string{"把关键义务改写成自己的待办清单，检查是否可执行。", "确认数据、模型输出和二次创作成果归属。", "对自动续费、单方变更和提前终止保持警惕。"},
			Risk:      "这类笔记只能提供风险意识，不能替代律师意见或正式法律咨询。",
			Example:   "采购 AI 服务时，应检查输入数据是否会用于训练、日志保存多久、出现泄露如何通知，以及供应商停服时数据如何导出。",
			Refs:      []string{"privacy_data_governance", "pricing_packaging", "event_sourcing_audit"},
		},
		{
			Key:       "health_sleep_cognition",
			Title:     "睡眠与认知表现",
			FolderKey: "science_folder",
			Tags:      []string{"health", "sleep", "cognition"},
			Field:     "科学与健康",
			Question:  "睡眠为什么会影响学习、情绪和决策质量",
			Scene:     "熬夜后人仍能完成简单任务，却更容易在复杂判断、情绪控制和长期记忆上出错。对学习者和开发者来说，睡眠不足带来的不是单纯困倦，而是注意力波动、工作记忆下降和错误监控变弱。",
			Method:    "记录睡眠时不只看时长，也看规律性、入睡前刺激、白天光照、咖啡因和运动。若要评估学习效率，可以同时记录复习表现、主观精力和当天重要决策，避免把短期兴奋误判为高产。",
			Practice:  []string{"重要学习任务优先安排在睡眠充足后的时段。", "睡前减少高刺激信息输入，让大脑有稳定收尾。", "把连续几天的表现趋势作为判断依据，不因一天波动下结论。"},
			Risk:      "健康笔记应避免过度医疗化，持续失眠或明显功能受损需要寻求专业帮助。",
			Example:   "如果某周 RAG 学习笔记产出很多但复习正确率下降，可能不是知识点太难，而是睡眠债影响了提取和整合能力。",
			Refs:      []string{"learning_science_review", "nutrition_behavior_change", "cognitive_bias_decision"},
		},
		{
			Key:       "nutrition_behavior_change",
			Title:     "营养行为改变笔记",
			FolderKey: "science_folder",
			Tags:      []string{"health", "nutrition", "behavior"},
			Field:     "科学与健康",
			Question:  "为什么知道健康知识并不等于能长期改变饮食行为",
			Scene:     "营养建议常被写成规则，但真实生活里有预算、时间、口味、社交和情绪压力。用户不是不知道要均衡饮食，而是很难在疲惫、赶时间或外卖选择有限时持续执行。",
			Method:    "行为改变应降低摩擦，而不是只增加意志力要求。可以从环境设计开始：预先准备高质量默认选项，减少高风险场景，记录触发因素。目标设小一些，先稳定一两个关键习惯，再扩大范围。",
			Practice:  []string{"用一周记录识别最常失败的餐次，而不是一次性改全部饮食。", "把健康选择放到更容易拿到的位置。", "允许计划内的弹性，避免一次偏离后彻底放弃。"},
			Risk:      "营养讨论容易滑向焦虑和道德评价，应关注可持续行为而不是羞耻感。",
			Example:   "对忙碌学生来说，与其要求每天精确计算热量，不如先保证早餐蛋白质、下午水分和深夜加餐替代方案。",
			Refs:      []string{"health_sleep_cognition", "learning_science_review", "product_metrics_north_star"},
		},
		{
			Key:       "bioinformatics_notes",
			Title:     "生物信息学数据流程入门",
			FolderKey: "science_folder",
			Tags:      []string{"biology", "data", "pipeline"},
			Field:     "科学与健康",
			Question:  "生命科学数据为什么特别强调流程可复现",
			Scene:     "生物信息学常处理测序、表达矩阵、注释库和统计模型。数据量大、步骤多、参数敏感，任何一次版本变化都可能影响结论。若只保存最终图表，后续很难判断差异来自生物信号还是流程变化。",
			Method:    "可复现流程应记录原始数据来源、软件版本、参数、参考数据库和中间产物。脚本化执行优于手工点选，环境隔离优于依赖本机状态。对关键结果保留质量控制指标和异常样本说明。",
			Practice:  []string{"每次分析生成一份运行记录，包含输入、参数和输出摘要。", "把样本元数据整理成结构化表格，避免文件名承载过多含义。", "图表结论必须能追溯到对应代码和数据版本。"},
			Risk:      "流程自动化不能替代领域解释，统计显著也不等于生物意义明确。",
			Example:   "比较两批样本表达差异时，应先检查批次效应、测序深度和样本标签，再讨论差异基因列表，否则很容易把技术噪声写成发现。",
			Refs:      []string{"data_warehouse_layering", "devops_release_practice", "learning_science_review"},
		},
		{
			Key:       "energy_transition_grid",
			Title:     "能源转型与电网调度",
			FolderKey: "science_folder",
			Tags:      []string{"energy", "grid", "climate"},
			Field:     "科学与健康",
			Question:  "高比例新能源接入后，电网为什么需要更多灵活性",
			Scene:     "风电和光伏的边际成本低，但输出受天气和时间影响。电力系统必须实时平衡供需，不能只看全年发电量。新能源比例提高后，调峰、储能、需求响应和跨区输电的重要性都会上升。",
			Method:    "分析能源转型时要把装机容量、实际发电、负荷曲线、备用容量和市场机制分开。灵活性资源可以来自电源侧、电网侧、负荷侧和储能侧。政策设计需要让这些资源获得合理收益。",
			Practice:  []string{"比较能源方案时同时看可靠性、成本、排放和建设周期。", "关注尖峰负荷和低新能源出力时段，而不是只看平均值。", "把用户侧需求响应视为系统资源，而不是单纯节电宣传。"},
			Risk:      "能源讨论容易陷入单一技术崇拜，忽略系统调度和社会接受度。",
			Example:   "一个城市推广分布式光伏后，仍需要考虑傍晚负荷高峰如何供应，可能通过储能、电价引导和可中断负荷共同解决。",
			Refs:      []string{"climate_adaptation_city", "public_policy_evidence", "supply_chain_resilience"},
		},
		{
			Key:       "climate_adaptation_city",
			Title:     "气候适应型城市设计",
			FolderKey: "science_folder",
			Tags:      []string{"climate", "urban", "resilience"},
			Field:     "科学与健康",
			Question:  "城市如何面对热浪、暴雨和极端天气成为常态",
			Scene:     "气候适应不是抽象环保口号，而是排水、遮阴、通风、应急、社区照护和基础设施韧性的组合。极端天气增加后，城市需要从“灾后修复”转向“提前降低脆弱性”。",
			Method:    "适应设计可以从风险地图开始，识别热岛、内涝、老旧社区、交通节点和弱势人群。再把灰色基础设施与蓝绿空间结合，利用海绵城市、树荫廊道、开放空间和预警系统分散风险。",
			Practice:  []string{"把学校、医院和养老设施纳入优先保护清单。", "用小尺度社区数据补充城市平均指标。", "改造项目同时评估日常舒适度和极端天气表现。"},
			Risk:      "适应项目若只集中在形象区域，可能加剧空间不平等。",
			Example:   "一条慢行街道的遮阴和透水铺装，不仅提升日常步行体验，也能在热浪和短时暴雨中降低健康风险。",
			Refs:      []string{"urban_mobility_planning", "energy_transition_grid", "public_policy_evidence"},
		},
		{
			Key:       "agriculture_soil_health",
			Title:     "土壤健康与农业韧性",
			FolderKey: "science_folder",
			Tags:      []string{"agriculture", "soil", "resilience"},
			Field:     "科学与健康",
			Question:  "为什么土壤不是简单承载作物的介质",
			Scene:     "土壤包含矿物、有机质、微生物、水分和结构。长期单一施肥、过度耕作或侵蚀会让产量在短期看似稳定，长期却降低保水、保肥和抗逆能力。农业韧性首先体现在土壤系统。",
			Method:    "观察土壤健康可以看有机质、团粒结构、入渗能力、生物活性和养分平衡。管理上结合轮作、覆盖作物、减少裸地、合理施肥和水土保持。不同地区要根据气候、作物和农户条件调整。",
			Practice:  []string{"记录田块历史，避免只看单季产量。", "把极端天气后的恢复速度作为韧性指标。", "推广技术时同时考虑农户成本、知识门槛和市场激励。"},
			Risk:      "农业方案若忽略地方经验和经济约束，很难长期落地。",
			Example:   "在容易干旱的地区，覆盖作物和秸秆还田可能提升土壤保水能力，但也需要评估病虫害、机械作业和短期收益变化。",
			Refs:      []string{"climate_adaptation_city", "supply_chain_resilience", "public_policy_evidence"},
		},
		{
			Key:       "behavioral_finance_notes",
			Title:     "行为金融与投资纪律",
			FolderKey: "society_folder",
			Tags:      []string{"finance", "behavior", "decision"},
			Field:     "人文社会",
			Question:  "投资中为什么情绪和制度设计常比预测能力更重要",
			Scene:     "投资者常高估自己识别拐点的能力，低估亏损厌恶、从众、近期偏差和过度交易的影响。即使掌握很多信息，也可能在市场波动时被情绪带着改变策略。",
			Method:    "投资纪律的核心是事前写规则：资产配置、再平衡条件、最大亏损承受、信息来源和不操作的理由。记录每次交易前的假设，复盘时看假设是否成立，而不是只看盈亏。",
			Practice:  []string{"把决策分成长期配置和短期判断，避免互相污染。", "对高情绪波动资产设置冷静期。", "定期复盘交易原因，识别重复犯错模式。"},
			Risk:      "这类笔记不构成投资建议，任何策略都需要结合个人风险承受能力。",
			Example:   "看到新能源新闻后想追涨，可以先检查该信息是否已反映在价格中、是否改变长期假设，以及仓位是否偏离原定配置。",
			Refs:      []string{"cognitive_bias_decision", "energy_transition_grid", "product_metrics_north_star"},
		},
		{
			Key:       "security_threat_modeling",
			Title:     "安全威胁建模入门",
			FolderKey: "ops_folder",
			Tags:      []string{"security", "threat-modeling", "ops"},
			Field:     "安全与运维",
			Question:  "如何在功能上线前识别最值得防的安全风险",
			Scene:     "安全问题如果等到事故后再处理，成本会远高于设计阶段。笔记系统涉及登录、导入文件、AI 调用、导出、删除和本地数据库，每个入口都有潜在滥用方式。",
			Method:    "威胁建模先画数据流和信任边界，再列资产、攻击者、入口和影响。可以使用 STRIDE 这类框架提醒自己关注伪造、篡改、抵赖、信息泄露、拒绝服务和权限提升。最终输出应是可执行的缓解清单。",
			Practice:  []string{"每个外部输入都明确解析、大小限制和错误处理。", "敏感操作记录审计事件，并要求合适权限。", "上线前检查默认配置，避免调试接口或密钥泄露。"},
			Risk:      "威胁建模不是一次性文档，系统新增能力后边界会变化。",
			Example:   "文件导入功能需要限制类型和大小，防止超大文件拖垮服务；同时要避免把导入内容直接拼进日志或提示词而暴露隐私。",
			Refs:      []string{"privacy_data_governance", "incident_response", "go_error_logging"},
		},
		{
			Key:       "incident_response",
			Title:     "线上事故响应复盘模板",
			FolderKey: "ops_folder",
			Tags:      []string{"ops", "incident", "postmortem"},
			Field:     "安全与运维",
			Question:  "事故发生后，团队如何尽快止损并把经验转化为系统改进",
			Scene:     "事故响应最怕两件事：一是所有人同时猜原因，没人负责用户影响；二是恢复后只写一句“已修复”，没有追踪根因和预防措施。复盘模板能让压力下的团队保持秩序。",
			Method:    "响应流程包括发现、分级、止损、沟通、恢复和复盘。复盘记录时间线、影响范围、根因、触发条件、检测缺口、改进项和负责人。重点讨论系统为何允许问题扩大，而不是追究某个人手滑。",
			Practice:  []string{"事故期间保持单一指挥和清晰沟通频道。", "先恢复用户影响，再做深度根因分析。", "复盘改进项必须有截止时间和验证方式。"},
			Risk:      "没有文化安全感的复盘会让人隐藏信息，最后只能得到表面原因。",
			Example:   "若 RAG 索引任务因外部模型超时拖慢保存接口，短期可隔离异步任务，长期要加超时、队列、降级和告警。",
			Refs:      []string{"observability_slo", "go_error_logging", "event_sourcing_audit"},
		},
		{
			Key:       "observability_slo",
			Title:     "可观测性与 SLO 设计",
			FolderKey: "ops_folder",
			Tags:      []string{"ops", "observability", "slo"},
			Field:     "安全与运维",
			Question:  "怎样从“有日志”走向“知道系统是否健康”",
			Scene:     "日志、指标和追踪是可观测性的材料，不是目标。系统真正需要回答的是：用户能否创建和保存笔记，搜索是否足够快，AI 问答是否可用，错误是否在影响扩大前被发现。",
			Method:    "SLO 从用户旅程定义，例如保存成功率、搜索 p95 延迟、AI 问答可用率。再设计 SLI 的采集方式、错误预算和告警阈值。告警应指向用户影响，而不是每个技术指标波动都叫醒人。",
			Practice:  []string{"为核心接口记录延迟、状态码、错误类别和依赖耗时。", "看板按用户旅程组织，而不是按机器资源堆图。", "告警消息包含影响、可能原因和首个排查入口。"},
			Risk:      "指标过多会让团队失去注意力，指标过少又会在事故中盲飞。",
			Example:   "保存接口 p95 突然升高时，追踪应能显示耗时来自 SQLite 写入、Markdown 渲染还是索引任务，而不是只看到整体变慢。",
			Refs:      []string{"incident_response", "go_error_logging", "server_test_strategy"},
		},
		{
			Key:       "devops_release_practice",
			Title:     "DevOps 发布与回滚实践",
			FolderKey: "ops_folder",
			Tags:      []string{"devops", "release", "ops"},
			Field:     "安全与运维",
			Question:  "如何让发布变成低风险、可重复、可回退的常规动作",
			Scene:     "发布风险不只来自代码 bug，也来自数据库迁移、配置变更、静态资源缓存、依赖服务和环境差异。手工发布越多，越依赖个人记忆，越难在压力下稳定执行。",
			Method:    "发布流程应自动化构建、测试、产物校验和部署记录。数据库迁移要可重复、可观测，回滚策略提前写好。对前端静态资源和后端 API 版本兼容性做检查，避免用户刷新后拿到不匹配版本。",
			Practice:  []string{"每次发布都生成版本号、变更摘要和负责人。", "高风险变更采用灰度或开关，先限制影响范围。", "回滚脚本和数据恢复路径要在演练中验证。"},
			Risk:      "没有回滚演练的回滚方案，事故时可能只是心理安慰。",
			Example:   "新增 note_blocks.level 字段时，迁移应先兼容旧数据，再发布使用新字段的前端，避免旧数据库打开后接口直接失败。",
			Refs:      []string{"server_test_strategy", "observability_slo", "bioinformatics_notes"},
		},
		{
			Key:       "supply_chain_resilience",
			Title:     "供应链韧性与风险地图",
			FolderKey: "ops_folder",
			Tags:      []string{"supply-chain", "risk", "operations"},
			Field:     "安全与运维",
			Question:  "组织如何识别单点依赖并提高供应链抗冲击能力",
			Scene:     "供应链风险既可能来自实体物流，也可能来自软件依赖、云服务、模型 API 和关键人员。系统平时看起来运行顺畅，真正的脆弱性常在某个供应商停服、地区中断或成本暴涨时暴露。",
			Method:    "风险地图先列关键资源、替代方案、切换时间、库存或缓存能力、合同约束和监控信号。再按影响和概率排序，给高风险依赖设计冗余、降级或退出机制。软件系统同样适用这个方法。",
			Practice:  []string{"对关键外部 API 设计超时、重试、熔断和备用路径。", "定期检查依赖许可证、维护状态和安全公告。", "把供应商合同和技术替代方案一起评估。"},
			Risk:      "韧性不是无限冗余，过度备份会让成本和复杂度吞掉收益。",
			Example:   "若默认 embedding 服务不可用，笔记保存不应失败；系统可以延迟索引，并在后台恢复后补建向量。",
			Refs:      []string{"energy_transition_grid", "contract_risk", "devops_release_practice"},
		},
		{
			Key:       "design_system_tokens",
			Title:     "设计系统 Token 与组件规范",
			FolderKey: "creation_folder",
			Tags:      []string{"design", "frontend", "system"},
			Field:     "设计与创作",
			Question:  "为什么设计系统要从颜色、间距和状态这些小单位开始",
			Scene:     "应用增长后，组件会被不同开发者不断复制修改。若没有 token，按钮、卡片、弹窗和标签很快出现相近但不一致的颜色、圆角、阴影和间距，界面会变得难维护。",
			Method:    "Token 把设计决策命名为可复用变量，如背景层级、文字层级、边框、焦点、危险色和间距尺度。组件规范再定义何时使用某个 token、哪些状态必须支持、哪些变体不允许新增。",
			Practice:  []string{"先整理高频组件，再扩展低频场景。", "每个组件写清默认、悬停、禁用、加载和错误状态。", "设计变量命名表达用途，而不是表达具体颜色值。"},
			Risk:      "设计系统若脱离实际页面，只会成为漂亮但没人遵守的文档。",
			Example:   "深色模式里，主按钮、危险按钮和图标按钮都应使用统一焦点样式，这样键盘用户在不同面板间移动时不会迷失。",
			Refs:      []string{"dark_mode_spec", "usability_walkthrough", "information_architecture"},
		},
		{
			Key:       "information_architecture",
			Title:     "信息架构与导航命名",
			FolderKey: "creation_folder",
			Tags:      []string{"ux", "navigation", "architecture"},
			Field:     "设计与创作",
			Question:  "为什么导航命名比视觉装饰更影响复杂应用的可用性",
			Scene:     "笔记系统的功能越来越多后，用户需要在主页、私人区域、搜索、AI、垃圾箱、模板、研究室和周报之间切换。若命名含糊或层级混乱，用户会记不住入口，只能靠试错。",
			Method:    "信息架构先从用户任务出发，把功能归到少数稳定区域。命名使用用户熟悉的词，避免内部实现名。导航层级不宜过深，高频入口可放在侧边栏或命令面板，低频配置进入设置。",
			Practice:  []string{"同一动作在不同位置保持同名，例如归档不要一处叫删除。", "空状态和错误提示也要使用同一套概念。", "通过搜索日志和访谈验证用户是否能预测入口位置。"},
			Risk:      "架构一旦频繁重排，用户的空间记忆会被反复打断。",
			Example:   "如果“周报生成”既属于 AI 又属于写作，导航可把它放在工作台，但在 AI 面板提供快捷入口，并保持同一个标题。",
			Refs:      []string{"user_research_interview", "design_system_tokens", "knowledge_base_roadmap"},
		},
		{
			Key:       "writing_research_notes",
			Title:     "研究写作中的资料卡片法",
			FolderKey: "creation_folder",
			Tags:      []string{"writing", "research", "notes"},
			Field:     "设计与创作",
			Question:  "如何把零散阅读转化为可写作的论证材料",
			Scene:     "研究写作最难的不是收集资料，而是从资料中形成问题意识和论证结构。若笔记只复制原文，写作时仍要重新理解；若每张资料卡都记录观点、证据、适用范围和自己的评论，后续组合会轻松很多。",
			Method:    "资料卡片可以包括来源、核心观点、证据类型、关键引用、反例、可连接主题和写作用途。整理时不要按阅读顺序堆放，而要按问题、概念和论证关系重新归类。",
			Practice:  []string{"每读完一段资料，写一句自己的解释，避免只摘录。", "为卡片添加可争论的问题，而不是只写关键词。", "写作前先排卡片结构，再决定章节标题。"},
			Risk:      "过度整理会变成拖延，卡片必须服务明确写作目标。",
			Example:   "准备论文功能补充章节时，可以把 RAG、周报、知识卡片和图谱分别做成资料卡，再用用户学习闭环串成一个论证。",
			Refs:      []string{"learning_science_review", "information_architecture", "rag_design"},
		},
		{
			Key:       "history_silk_road_exchange",
			Title:     "丝绸之路与技术文化交流",
			FolderKey: "society_folder",
			Tags:      []string{"history", "culture", "exchange"},
			Field:     "人文社会",
			Question:  "历史上的交流网络如何改变技术、商业和知识传播",
			Scene:     "丝绸之路不是一条单线道路，而是跨越绿洲、草原、海路和城市节点的交流网络。商品、宗教、工艺、语言和制度在流动中被重新解释，技术传播也常伴随本地化改造。",
			Method:    "观察历史交流要同时看路线、节点、媒介和权力关系。路线决定连接可能性，节点形成集散和翻译，媒介影响知识如何被保存，权力关系则决定哪些交流被鼓励或阻断。",
			Practice:  []string{"不要把技术传播理解为单向输入，应关注本地再创造。", "比较不同节点城市的角色，识别商业与文化功能差异。", "把历史案例与当代知识网络联系起来，观察相似结构。"},
			Risk:      "宏大叙事容易忽略普通商人、工匠和译者的具体作用。",
			Example:   "一项造纸或染织技术在不同地区落地时，会受原料、审美、市场和制度影响，最后形成不同工艺传统。",
			Refs:      []string{"media_literacy_algorithm", "writing_research_notes", "urban_mobility_planning"},
		},
		{
			Key:       "media_literacy_algorithm",
			Title:     "算法时代的媒体素养",
			FolderKey: "society_folder",
			Tags:      []string{"media", "algorithm", "literacy"},
			Field:     "人文社会",
			Question:  "信息流平台为什么会改变我们判断可信度的方式",
			Scene:     "过去人们更多通过编辑、出版社或熟人网络接触信息，现在平台算法会根据互动预测持续推送内容。高情绪、高冲突和强立场信息更容易获得注意，用户也更容易把“频繁看到”误认为“更真实”。",
			Method:    "媒体素养需要检查来源、证据、动机、发布时间和传播路径。面对陌生结论，先区分事实陈述、观点判断和情绪动员，再寻找原始材料或多方交叉验证。也要观察自己为何愿意相信某条信息。",
			Practice:  []string{"看到强烈情绪内容时延迟转发，先找原始来源。", "关注推荐系统如何根据自己的行为塑造信息环境。", "把重要判断写成决策记录，避免被即时情绪带走。"},
			Risk:      "过度怀疑会滑向犬儒，合理目标是提高验证能力，而不是否认所有公共信息。",
			Example:   "关于能源转型的短视频若只展示单个事故，应进一步查找系统统计、政策背景和技术约束，再形成判断。",
			Refs:      []string{"cognitive_bias_decision", "energy_transition_grid", "history_silk_road_exchange"},
		},
	}
}
