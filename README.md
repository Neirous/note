# RAG Note (Go + Vue)

基于 `Go + SQLite + Vue 3 + Vite` 的笔记系统，当前实现：

- Notion 风格深色布局（左侧导航 + 主页卡片 + 页面编辑区）
- 左侧支持折叠、顶部快速新建页面、主页/搜索/AI/私人/垃圾箱布局
- 搜索弹窗（`Ctrl/Cmd+K`）与最近访问横向滑动卡片（带日期）
- 私人区域树形嵌套（父子页面）与“新建文件夹/新建页面”
- Markdown 笔记编辑、预览、保存
- 笔记 CRUD、父子页面、标签筛选、关键词搜索
- 归档、恢复
- 复制页面、导出 Markdown
- 页面内引用跳转：支持 Markdown 内链 `note://id`，并可把 `[[页面标题]]` 解析为内部链接
- 已有页面自动保存（基于 Vue `watch` + 防抖）
- 右下角 AI 悬浮窗（简化版 Notion AI：仅问答）
- 垃圾箱弹窗（归档列表、恢复、永久删除）
- 快捷键：`Ctrl/Cmd+S` 保存笔记
- 后端保留块能力（`note_blocks` 表与块 API）
- RAG：分块、向量检索、基于上下文回答
- LLM 提供方切换：`dashscope`（OpenAI 兼容）或 `ollama`

## 1. 启动

### 1.1 构建前端

```powershell
Push-Location web/frontend
npm install
npm run build
Pop-Location
```

前端产物输出到 `web/static`，由 Go 服务托管。

前端本地开发（可选）：

```powershell
Push-Location web/frontend
npm run dev
Pop-Location
```

### 1.2 启动后端

```powershell
go mod tidy
go test ./...
go run ./cmd/server
```

访问：`http://localhost:8080`

## 2. 模型配置

### 2.1 DashScope（默认）

```powershell
$env:LLM_PROVIDER="dashscope"
$env:OPENAI_BASE_URL="https://dashscope.aliyuncs.com/compatible-mode/v1"
$env:OPENAI_API_KEY=""
$env:OPENAI_CHAT_MODEL="qwen-plus"
$env:OPENAI_EMBED_MODEL="text-embedding-v3"
go run ./cmd/server
```

```
LLM_PROVIDER="dashscope" \
OPENAI_BASE_URL="https://dashscope.aliyuncs.com/compatible-mode/v1" \
OPENAI_API_KEY="" \
OPENAI_CHAT_MODEL="qwen-plus" \
OPENAI_EMBED_MODEL="text-embedding-v3" \
go run ./cmd/server
```
### 2.2 Ollama

```powershell
$env:LLM_PROVIDER="ollama"
$env:OLLAMA_BASE_URL="http://localhost:11434"
$env:OLLAMA_GEN_MODEL="qwen2.5:7b"
$env:OLLAMA_EMBED_MODEL="nomic-embed-text"
go run ./cmd/server
```

说明：
- `OPENAI_API_KEY` 为空时，笔记保存仍可用，RAG 请求会返回错误信息。
- 不要把真实 Key 写入源码文件，使用环境变量。

### 2.3 录入演示数据（多目录 + 多篇 600+ 字笔记）

```powershell
$env:OPENAI_BASE_URL="https://dashscope.aliyuncs.com/compatible-mode/v1"
$env:OPENAI_API_KEY="你的DashScopeKey"
$env:OPENAI_CHAT_MODEL="qwen-plus"
$env:OPENAI_EMBED_MODEL="text-embedding-v3"
go run ./cmd/seednotes
```

说明：
- 会创建 9 个目录页和 47 篇 600+ 字较长演示笔记，覆盖后端、AI、前端、数据、产品、人文、科学健康、安全运维、设计创作等领域。
- 演示笔记之间包含用户可读的双链内部引用，正文里保存为 `[[页面标题]]`，预览时会解析成真实内部跳转。
- 反复执行会按“标题 + 父页面”更新，不会无限重复新增。
- 若已配置 API Key，会顺带重建这些页面的 RAG 索引。

## 3. API

- `GET /api/notes?q=关键词&tag=标签&include_archived=1&archived=1&lite=1`
- `GET /api/tags`
- `POST /api/notes`
- `GET /api/notes/{id}`
- `PUT /api/notes/{id}`
- `PATCH /api/notes/{id}/pin`
- `PATCH /api/notes/{id}/archive`
- `POST /api/notes/{id}/duplicate`
- `GET /api/notes/{id}/export.md`
- `GET /api/notes/{id}/blocks`
- `PUT /api/notes/{id}/blocks`
- `DELETE /api/notes/{id}`
- `POST /api/render`
- `POST /api/rag/search`
- `POST /api/rag/ask`

`PUT /api/notes/{id}/blocks` 请求体示例：

```json
{
  "blocks": [
    { "type": "heading1", "content": "周计划", "checked": false, "level": 0 },
    { "type": "todo", "content": "完成接口联调", "checked": true, "level": 1 },
    { "type": "code", "content": "go test ./...", "checked": false, "level": 0 }
  ]
}
```

## 4. 官方资料

- Vue SFC（官方）：<https://vuejs.org/guide/scaling-up/sfc>
- Vue 状态管理（官方）：<https://vuejs.org/guide/scaling-up/state-management.html>
- Vue Event Handling（官方）：<https://vuejs.org/guide/essentials/event-handling.html>
- Vite Guide（官方）：<https://vite.dev/guide/>
- Web Storage API（MDN）：<https://developer.mozilla.org/docs/Web/API/Web_Storage_API>
- HTML Drag and Drop API（MDN）：<https://developer.mozilla.org/docs/Web/API/HTML_Drag_and_Drop_API>
- CommonMark（官方）：<https://spec.commonmark.org/current/>
- Notion Markdown & 快捷键（官方）：<https://www.notion.com/help/markdown-and-keyboard-shortcuts>
- DashScope OpenAI 兼容（官方）：<https://help.aliyun.com/zh/model-studio/compatibility-of-openai-with-dashscope>
- DashScope 模型列表（官方）：<https://help.aliyun.com/zh/model-studio/getting-started/models>
- OpenAI Embeddings API（官方）：<https://platform.openai.com/docs/api-reference/embeddings>
- OpenAI Chat Completions API（官方）：<https://platform.openai.com/docs/api-reference/chat/create>
- HTTP 语义（RFC 9110）：<https://www.rfc-editor.org/rfc/rfc9110>
- Go `database/sql`（官方）：<https://pkg.go.dev/database/sql>
- SQLite `ALTER TABLE`（官方）：<https://www.sqlite.org/lang_altertable.html>
- SQLite Query Planner（官方）：<https://www.sqlite.org/queryplanner.html>
- SQLite FTS5（官方）：<https://www.sqlite.org/fts5.html>
- Chi：<https://github.com/go-chi/chi>
- Goldmark：<https://github.com/yuin/goldmark>
- Ollama API：<https://docs.ollama.com/api>
- RAG 论文（NeurIPS 2020）：<https://papers.nips.cc/paper/2020/hash/6b493230205f780e1bc26945df7481e5-Abstract.html>
# ci trigger
