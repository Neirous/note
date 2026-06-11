package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"note/internal/store"
)

type noteUpsertRequest struct {
	Title    string   `json:"title"`
	Markdown string   `json:"markdown"`
	ParentID *int64   `json:"parent_id"`
	Tags     []string `json:"tags"`
}

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := store.NoteFilter{
		Query:           strings.TrimSpace(r.URL.Query().Get("q")),
		Tag:             strings.TrimSpace(r.URL.Query().Get("tag")),
		IncludeArchived: parseBool(r.URL.Query().Get("include_archived")),
		OnlyArchived:    parseBool(r.URL.Query().Get("archived")),
	}
	lite := parseBool(r.URL.Query().Get("lite"))

	var notes []store.Note
	var err error
	if lite {
		notes, err = s.store.ListNotesLite(ctx, filter)
	} else {
		notes, err = s.store.ListNotes(ctx, filter)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if notes == nil {
		notes = []store.Note{}
	}
	writeJSON(w, http.StatusOK, notes)
}

func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tags, err := s.store.ListDistinctTags(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if tags == nil {
		tags = []string{}
	}
	writeJSON(w, http.StatusOK, tags)
}

func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	if tag == "" {
		var req struct {
			Tag string `json:"tag"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			tag = strings.TrimSpace(req.Tag)
		}
	}
	if tag == "" {
		writeErrMsg(w, http.StatusBadRequest, "tag is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := s.store.DeleteTag(ctx, tag); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":  true,
		"tag": tag,
	})
}

func (s *Server) handleGetNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	n, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleDuplicateNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	src, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	dupTitle := "Copy of " + strings.TrimSpace(src.Title)
	if strings.TrimSpace(src.Title) == "" {
		dupTitle = "Copy of Untitled"
	}

	created, err := s.store.CreateNote(ctx, store.NoteInput{
		ParentID: src.ParentID,
		Title:    dupTitle,
		Markdown: src.Markdown,
		HTML:     src.HTML,
		Tags:     src.Tags,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	srcBlocks, err := s.store.ListNoteBlocks(ctx, src.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if len(srcBlocks) > 0 {
		inputs := make([]store.NoteBlockInput, 0, len(srcBlocks))
		for _, b := range srcBlocks {
			inputs = append(inputs, store.NoteBlockInput{
				Type: b.Type, Content: b.Content, Checked: b.Checked, Level: b.Level,
			})
		}
		if err := s.store.ReplaceNoteBlocks(ctx, created.ID, inputs); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
	}

	if err := s.rag.IndexNote(ctx, created.ID, created.Markdown); err != nil {
		log.Printf("index note %d warning: %v", created.ID, err)
		writeJSON(w, http.StatusCreated, map[string]any{
			"note": created, "index_warning": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleExportMarkdown(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	n, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	filename := sanitizeFilename(n.Title)
	if filename == "" {
		filename = "note"
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%d.md"`, filename, n.ID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(n.Markdown))
}

func (s *Server) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	n, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	blocks, err := s.store.ListNoteBlocks(ctx, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if len(blocks) == 0 && strings.TrimSpace(n.Markdown) != "" {
		blocks = parseMarkdownToBlocks(id, n.Markdown)
	}
	if blocks == nil {
		blocks = []store.NoteBlock{}
	}
	writeJSON(w, http.StatusOK, blocks)
}

func (s *Server) handleReplaceBlocks(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct {
		Blocks []store.NoteBlockInput `json:"blocks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Blocks = normalizeBlockInputs(req.Blocks)

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	n, err := s.store.GetNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.store.ReplaceNoteBlocks(ctx, id, req.Blocks); errors.Is(err, store.ErrInvalidBlock) {
		writeErrMsg(w, http.StatusBadRequest, "invalid block type")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	blocks, err := s.store.ListNoteBlocks(ctx, id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	markdown := blocksToMarkdown(blocks)
	resolvedMarkdown, err := s.resolveStoredInternalLinks(ctx, markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	html, err := s.renderMarkdown(resolvedMarkdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	updated, err := s.store.UpdateNote(ctx, id, store.NoteInput{
		ParentID: n.ParentID, Title: n.Title, Markdown: markdown, HTML: html, Tags: n.Tags,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.rag.IndexNote(ctx, updated.ID, updated.Markdown); err != nil {
		log.Printf("index note %d warning: %v", updated.ID, err)
		writeJSON(w, http.StatusOK, map[string]any{"note": updated, "blocks": blocks, "index_warning": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"note": updated, "blocks": blocks})
}

func (s *Server) handlePinNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct{ Value bool `json:"value"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	n, err := s.store.SetPinned(ctx, id, req.Value)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if errors.Is(err, store.ErrInvalidState) {
		writeErrMsg(w, http.StatusBadRequest, "cannot pin archived note")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleArchiveNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct{ Value bool `json:"value"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	n, err := s.store.SetArchived(ctx, id, req.Value)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleSetNoteStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req struct{ Status string `json:"status"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "unfinished" && status != "completed" {
		writeErrMsg(w, http.StatusBadRequest, "status must be unfinished or completed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	n, err := s.store.SetNoteStatus(ctx, id, status)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	err := s.store.DeleteNote(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	var req noteUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = "Untitled"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := s.validateParentFolder(ctx, req.ParentID); errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusBadRequest, "parent note not found")
		return
	} else if err != nil {
		writeErrMsg(w, http.StatusBadRequest, err.Error())
		return
	}
	resolvedMarkdown, err := s.resolveStoredInternalLinks(ctx, req.Markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	html, err := s.renderMarkdown(resolvedMarkdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, err := s.store.CreateNote(ctx, store.NoteInput{
		ParentID: req.ParentID, Title: req.Title, Markdown: req.Markdown, HTML: html, Tags: req.Tags,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.rag.IndexNote(ctx, n.ID, n.Markdown); err != nil {
		log.Printf("index note %d warning: %v", n.ID, err)
		writeJSON(w, http.StatusCreated, map[string]any{"note": n, "index_warning": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, n)
}

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var req noteUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = "Untitled"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if req.ParentID != nil && *req.ParentID == id {
		writeErrMsg(w, http.StatusBadRequest, "parent_id cannot be self")
		return
	}
	if err := s.validateParentFolder(ctx, req.ParentID); errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusBadRequest, "parent note not found")
		return
	} else if err != nil {
		writeErrMsg(w, http.StatusBadRequest, err.Error())
		return
	}
	resolvedMarkdown, err := s.resolveStoredInternalLinks(ctx, req.Markdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	html, err := s.renderMarkdown(resolvedMarkdown)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	n, err := s.store.UpdateNote(ctx, id, store.NoteInput{
		ParentID: req.ParentID, Title: req.Title, Markdown: req.Markdown, HTML: html, Tags: req.Tags,
	})
	if errors.Is(err, store.ErrNotFound) {
		writeErrMsg(w, http.StatusNotFound, "note not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}

	if err := s.rag.IndexNote(ctx, n.ID, n.Markdown); err != nil {
		log.Printf("index note %d warning: %v", n.ID, err)
		writeJSON(w, http.StatusOK, map[string]any{"note": n, "index_warning": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// ---- Block helpers ----

func blocksToMarkdown(blocks []store.NoteBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		content := strings.TrimSpace(b.Content)
		if content == "" {
			continue
		}
		level := b.Level
		if level < 0 {
			level = 0
		}
		indent := strings.Repeat("  ", level)
		switch b.Type {
		case "heading1":
			parts = append(parts, "# "+content)
		case "todo":
			prefix := "- [ ] "
			if b.Checked {
				prefix = "- [x] "
			}
			parts = append(parts, indent+prefix+content)
		case "code":
			parts = append(parts, "```\n"+content+"\n```")
		case "quote":
			lines := strings.Split(content, "\n")
			for i := range lines {
				lines[i] = strings.Repeat("> ", level+1) + lines[i]
			}
			parts = append(parts, strings.Join(lines, "\n"))
		case "table":
			parts = append(parts, content)
		default:
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n\n")
}

func normalizeBlockInputs(in []store.NoteBlockInput) []store.NoteBlockInput {
	if len(in) == 0 {
		return in
	}
	out := make([]store.NoteBlockInput, 0, len(in))
	prevLevel := 0
	for i, b := range in {
		level := b.Level
		if level < 0 {
			level = 0
		}
		if level > 6 {
			level = 6
		}
		if i == 0 {
			level = 0
		} else if level > prevLevel+1 {
			level = prevLevel + 1
		}
		b.Level = level
		prevLevel = level
		out = append(out, b)
	}
	return out
}

func parseMarkdownToBlocks(noteID int64, markdown string) []store.NoteBlock {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var (
		out []store.NoteBlock
		pos int
	)
	for i := 0; i < len(lines); {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			i++
			continue
		}

		add := func(typ, content string, checked bool) {
			out = append(out, store.NoteBlock{
				NoteID: noteID, Position: pos, Level: 0, Type: typ, Content: content, Checked: checked,
			})
			pos++
		}

		if strings.HasPrefix(line, "```") {
			i++
			var code []string
			for i < len(lines) && strings.TrimSpace(lines[i]) != "```" {
				code = append(code, lines[i])
				i++
			}
			if i < len(lines) {
				i++
			}
			add("code", strings.TrimSpace(strings.Join(code, "\n")), false)
			continue
		}
		if strings.HasPrefix(line, "# ") {
			add("heading1", strings.TrimSpace(strings.TrimPrefix(line, "# ")), false)
			i++
			continue
		}
		leadingSpaces := len(lines[i]) - len(strings.TrimLeft(lines[i], " "))
		level := leadingSpaces / 2
		todoLine := strings.TrimLeft(lines[i], " ")
		if strings.HasPrefix(todoLine, "- [ ] ") || strings.HasPrefix(strings.ToLower(todoLine), "- [x] ") {
			checked := strings.HasPrefix(strings.ToLower(todoLine), "- [x] ")
			content := strings.TrimSpace(todoLine[6:])
			add("todo", content, checked)
			out[len(out)-1].Level = level
			i++
			continue
		}
		if strings.HasPrefix(line, "> ") {
			var quoteLines []string
			for i < len(lines) {
				cur := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(cur, "> ") {
					break
				}
				quoteLines = append(quoteLines, strings.TrimPrefix(cur, "> "))
				i++
			}
			add("quote", strings.Join(quoteLines, "\n"), false)
			continue
		}
		if strings.HasPrefix(line, "|") {
			var tableLines []string
			for i < len(lines) {
				cur := strings.TrimSpace(lines[i])
				if !strings.HasPrefix(cur, "|") {
					break
				}
				tableLines = append(tableLines, lines[i])
				i++
			}
			add("table", strings.TrimSpace(strings.Join(tableLines, "\n")), false)
			continue
		}

		var para []string
		for i < len(lines) {
			cur := strings.TrimSpace(lines[i])
			if cur == "" {
				break
			}
			if strings.HasPrefix(cur, "# ") || strings.HasPrefix(cur, "```") ||
				strings.HasPrefix(cur, "> ") || strings.HasPrefix(cur, "|") ||
				strings.HasPrefix(cur, "- [ ] ") || strings.HasPrefix(strings.ToLower(cur), "- [x] ") {
				break
			}
			para = append(para, lines[i])
			i++
		}
		add("paragraph", strings.TrimSpace(strings.Join(para, "\n")), false)
	}
	return out
}
