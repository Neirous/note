package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"note/internal/store"
)

var wikiLinkRE = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
var markdownLinkRE = regexp.MustCompile(`(!?)\[([^\]]+)\]\(([^)]+)\)`)
var externalLinkRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z\d+.-]*:`)

func (s *Server) handleRenderMarkdown(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Markdown string `json:"markdown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	html, err := s.renderMarkdown(req.Markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"html": html})
}

// ---- Optimize ----

type optimizeNoteRequest struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

type optimizeReference struct {
	NoteID int64  `json:"note_id"`
	Title  string `json:"title"`
	Path   string `json:"path"`
}

type optimizeContextNote struct {
	ID       int64
	Title    string
	Path     string
	Markdown string
}

func (s *Server) handleOptimizeNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if s.rag == nil {
		writeErrMsg(w, http.StatusServiceUnavailable, "ai optimizer not configured")
		return
	}

	var req optimizeNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	current, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSpace(current.Title)
	}
	if title == "" {
		title = "Untitled"
	}

	markdown := req.Markdown
	if strings.TrimSpace(markdown) == "" {
		markdown = current.Markdown
	}

	refs, docs, err := s.collectOptimizeContext(ctx, id, markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	prompt := buildOptimizePrompt(title, markdown, docs)
	optimized, err := s.rag.Generate(ctx, prompt)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}

	optimized = unwrapMarkdownFence(optimized)
	if strings.TrimSpace(optimized) == "" {
		writeErrMsg(w, http.StatusBadGateway, "empty optimization result")
		return
	}

	resolvedMarkdown, err := s.resolveStoredInternalLinks(ctx, optimized)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	html, err := s.renderMarkdown(resolvedMarkdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"markdown": optimized, "html": html, "references": refs,
	})
}

func (s *Server) renderMarkdown(markdown string) (string, error) {
	var buf bytes.Buffer
	if err := s.md.Convert([]byte(markdown), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *Server) resolveStoredInternalLinks(ctx context.Context, markdown string) (string, error) {
	notes, err := s.store.ListNotesLite(ctx, store.NoteFilter{IncludeArchived: true})
	if err != nil {
		return "", err
	}

	byID := make(map[int64]store.Note, len(notes))
	byTitle := make(map[string]int64, len(notes))
	for _, note := range notes {
		byID[note.ID] = note
		key := strings.ToLower(strings.TrimSpace(note.Title))
		if key != "" {
			byTitle[key] = note.ID
		}
	}

	byPath := make(map[string]int64, len(notes))
	for _, note := range notes {
		key := normalizeLinkPath(noteRoutePath(note, byID))
		if key == "" {
			continue
		}
		byPath[key] = note.ID
	}

	resolved := wikiLinkRE.ReplaceAllStringFunc(markdown, func(match string) string {
		sub := wikiLinkRE.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		name := strings.TrimSpace(sub[1])
		if name == "" {
			return match
		}
		id := byTitle[strings.ToLower(name)]
		if id == 0 {
			return match
		}
		return fmt.Sprintf("[%s](note://%d)", name, id)
	})

	resolved = markdownLinkRE.ReplaceAllStringFunc(resolved, func(match string) string {
		sub := markdownLinkRE.FindStringSubmatch(match)
		if len(sub) < 4 {
			return match
		}
		if sub[1] == "!" {
			return match
		}
		target := strings.TrimSpace(sub[3])
		if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "note://") {
			return match
		}
		if externalLinkRE.MatchString(target) {
			return match
		}
		id := byPath[normalizeLinkPath(target)]
		if id == 0 {
			return match
		}
		return fmt.Sprintf("[%s](note://%d)", sub[2], id)
	})

	return resolved, nil
}

func noteRoutePath(note store.Note, byID map[int64]store.Note) string {
	parts := make([]string, 0, 4)
	cur := note
	seen := map[int64]struct{}{}
	for {
		if _, ok := seen[cur.ID]; ok {
			break
		}
		seen[cur.ID] = struct{}{}
		title := strings.TrimSpace(cur.Title)
		if title != "" {
			parts = append(parts, title)
		}
		if cur.ParentID == nil {
			break
		}
		parent, ok := byID[*cur.ParentID]
		if !ok || !noteHasTag(parent, "folder") {
			break
		}
		cur = parent
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, "/")
}

func normalizeLinkPath(raw string) string {
	parts := strings.Split(strings.TrimSpace(raw), "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		decoded, err := url.PathUnescape(part)
		if err == nil {
			part = decoded
		}
		out = append(out, strings.ToLower(part))
	}
	return strings.Join(out, "/")
}

func (s *Server) collectOptimizeContext(ctx context.Context, selfID int64, markdown string) ([]optimizeReference, []optimizeContextNote, error) {
	notes, err := s.store.ListNotes(ctx, store.NoteFilter{IncludeArchived: true})
	if err != nil {
		return nil, nil, err
	}

	byID := make(map[int64]store.Note, len(notes))
	byTitle := make(map[string]int64, len(notes))
	for _, note := range notes {
		byID[note.ID] = note
		key := strings.ToLower(strings.TrimSpace(note.Title))
		if key != "" {
			byTitle[key] = note.ID
		}
	}
	byPath := make(map[string]int64, len(notes))
	for _, note := range notes {
		path := normalizeLinkPath(noteRoutePath(note, byID))
		if path != "" {
			byPath[path] = note.ID
		}
	}

	appendID := func(out []int64, seen map[int64]struct{}, sid int64) []int64 {
		if sid <= 0 || sid == selfID {
			return out
		}
		if _, ok := seen[sid]; ok {
			return out
		}
		if _, ok := byID[sid]; !ok {
			return out
		}
		seen[sid] = struct{}{}
		return append(out, sid)
	}

	seen := map[int64]struct{}{}
	var ids []int64
	resolvedWiki := wikiLinkRE.FindAllStringSubmatch(markdown, -1)
	for _, sub := range resolvedWiki {
		if len(sub) < 2 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(sub[1]))
		if name == "" {
			continue
		}
		ids = appendID(ids, seen, byTitle[name])
		if len(ids) >= 8 {
			break
		}
	}

	if len(ids) < 8 {
		resolvedLinks := markdownLinkRE.FindAllStringSubmatch(markdown, -1)
		for _, sub := range resolvedLinks {
			if len(sub) < 4 || sub[1] == "!" {
				continue
			}
			target := strings.TrimSpace(sub[3])
			if target == "" || strings.HasPrefix(target, "#") {
				continue
			}
			var sid int64
			switch {
			case strings.HasPrefix(target, "note://"):
				rawID := strings.TrimPrefix(target, "note://")
				sid, _ = strconv.ParseInt(rawID, 10, 64)
			case externalLinkRE.MatchString(target):
				continue
			default:
				sid = byPath[normalizeLinkPath(target)]
			}
			ids = appendID(ids, seen, sid)
			if len(ids) >= 8 {
				break
			}
		}
	}

	refs := make([]optimizeReference, 0, len(ids))
	docs := make([]optimizeContextNote, 0, len(ids))
	for _, sid := range ids {
		note := byID[sid]
		title := strings.TrimSpace(note.Title)
		if title == "" {
			title = fmt.Sprintf("Untitled#%d", note.ID)
		}
		path := noteRoutePath(note, byID)
		refs = append(refs, optimizeReference{NoteID: note.ID, Title: title, Path: path})
		docs = append(docs, optimizeContextNote{ID: note.ID, Title: title, Path: path, Markdown: note.Markdown})
	}
	return refs, docs, nil
}

func buildOptimizePrompt(title, markdown string, docs []optimizeContextNote) string {
	var sb strings.Builder
	sb.WriteString("你是中文笔记整理助手。你的任务是把一篇已有笔记整理成更易读、更清晰、更适合复习的 Markdown 笔记。\n")
	sb.WriteString("必须遵守以下规则：\n")
	sb.WriteString("1. 只能重组、润色和排版已有内容，不得捏造新事实。\n")
	sb.WriteString("2. 保留原有技术结论、代码片段、链接、内部链接和引用含义。\n")
	sb.WriteString("3. 优先通过标题、列表、表格、引用块、强调、分段来提升可读性。\n")
	sb.WriteString("4. 如果原文结构混乱，可以重排章节顺序，但不要删掉关键信息。\n")
	sb.WriteString("5. 输出必须是完整 Markdown 正文，不要解释，不要加```markdown代码围栏。\n\n")
	sb.WriteString("当前笔记标题：")
	sb.WriteString(title)
	sb.WriteString("\n\n当前笔记原文：\n")
	sb.WriteString(markdown)
	if len(docs) > 0 {
		sb.WriteString("\n\n以下是当前笔记中链接到的关联笔记，可作为整理时的补充上下文。你可以参考它们来改进结构和表达，但仍然不能凭空新增事实：\n")
		for i, doc := range docs {
			sb.WriteString(fmt.Sprintf("\n[关联笔记 %d]\n标题：%s\n路径：%s\n内容：\n%s\n", i+1, doc.Title, doc.Path, doc.Markdown))
		}
	}
	sb.WriteString("\n请直接输出优化后的完整 Markdown。")
	return sb.String()
}

func unwrapMarkdownFence(raw string) string {
	text := strings.TrimSpace(raw)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) < 3 {
		return text
	}
	first := strings.TrimSpace(lines[0])
	last := strings.TrimSpace(lines[len(lines)-1])
	if !strings.HasPrefix(first, "```") || last != "```" {
		return text
	}
	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
}
