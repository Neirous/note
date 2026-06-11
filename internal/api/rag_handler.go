package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"note/internal/rag"
)

func (s *Server) handleRAGSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query  string `json:"query"`
		TopK   int    `json:"top_k"`
		NoteID *int64 `json:"note_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	result, err := s.rag.SearchWithOptions(ctx, req.Query, req.TopK, rag.SearchOptions{
		AnchorNoteID: valueOrZero(req.NoteID),
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRAGAsk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query  string `json:"query"`
		TopK   int    `json:"top_k"`
		NoteID *int64 `json:"note_id"`
		Mode   string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrMsg(w, http.StatusBadRequest, "invalid json")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if strings.EqualFold(req.Mode, "library") {
		searchResult, searchErr := s.rag.SearchWithOptions(ctx, req.Query, req.TopK, rag.SearchOptions{
			AnchorNoteID: valueOrZero(req.NoteID),
		})
		if searchErr != nil {
			writeErr(w, http.StatusBadGateway, searchErr)
			return
		}
		answer, err := s.rag.Generate(ctx, buildLibraryRAGPrompt(req.Query, searchResult.Results, valueOrZero(req.NoteID)))
		if err != nil {
			writeErr(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, rag.AskResult{
			Query: req.Query, Answer: answer, Contexts: searchResult.Results,
		})
		return
	}
	if strings.EqualFold(req.Mode, "assistant") {
		opts := rag.SearchOptions{AnchorNoteID: valueOrZero(req.NoteID)}
		searchResult, searchErr := s.rag.SearchWithOptions(ctx, req.Query, req.TopK, opts)
		contexts := []rag.ChunkWithScore{}
		if searchErr == nil {
			contexts = searchResult.Results
		}
		answer, err := s.rag.Generate(ctx, buildAssistantRAGPrompt(req.Query, contexts, opts.AnchorNoteID))
		if err != nil {
			if searchErr != nil {
				writeErr(w, http.StatusBadGateway, searchErr)
				return
			}
			writeErr(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, rag.AskResult{
			Query: req.Query, Answer: answer, Contexts: contexts,
		})
		return
	}
	result, err := s.rag.AskWithOptions(ctx, req.Query, req.TopK, rag.SearchOptions{
		AnchorNoteID: valueOrZero(req.NoteID),
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func buildLibraryRAGPrompt(query string, contexts []rag.ChunkWithScore, anchorNoteID int64) string {
	var sb strings.Builder
	sb.WriteString("你是全库问答助手，职责是回答用户关于自己笔记库的问题。\n")
	sb.WriteString("回答边界：必须基于用户问题中显式提供的材料和检索到的全库笔记片段回答；不要凭空补充笔记中没有的事实。\n")
	sb.WriteString("适合处理的问题包括：查找某个概念或结论、按主题总结多篇笔记、比较不同方案、梳理待继续推进的主题、指出证据不足或缺失的笔记。\n")
	sb.WriteString("如果上下文不足，先说明'笔记库里没有足够依据'，再给出可以继续搜索或补充的关键词。不要替用户写一段无来源的泛泛建议。\n")
	if anchorNoteID > 0 {
		sb.WriteString("当前打开笔记如出现在上下文中会标记为 CURRENT_NOTE；提到当前笔记时以 CURRENT_NOTE 为准。\n")
	}
	sb.WriteString("\n用户问题：\n")
	sb.WriteString(strings.TrimSpace(query))
	sb.WriteString("\n\n检索到的全库笔记片段：\n")
	if len(contexts) == 0 {
		sb.WriteString("无可用检索片段。\n")
	} else {
		for i, c := range contexts {
			role := "LIBRARY_NOTE"
			if anchorNoteID > 0 && c.NoteID == anchorNoteID {
				role = "CURRENT_NOTE"
			}
			title := strings.TrimSpace(c.NoteTitle)
			titlePart := ""
			if title != "" {
				titlePart = fmt.Sprintf(" title=%q", title)
			}
			sb.WriteString(fmt.Sprintf("[%d] %s note_id=%d%s chunk_index=%d score=%.4f\n%s\n\n",
				i+1, role, c.NoteID, titlePart, c.Index, c.Score, c.Content))
		}
	}
	sb.WriteString("请用中文回答。先直接回答问题，再用'依据'简要说明来自哪些笔记标题；除非用户要求，不要输出内部 chunk 编号。")
	return sb.String()
}

func buildAssistantRAGPrompt(query string, contexts []rag.ChunkWithScore, anchorNoteID int64) string {
	var sb strings.Builder
	sb.WriteString("你是独立的工作台 AI 助手，不只是单篇笔记里的问答功能。\n")
	sb.WriteString("你可以处理用户直接输入的计划、临时材料、附件内容，也可以参考检索到的笔记。\n")
	sb.WriteString("优先级：1) 用户问题里显式给出的内容和【助手工作台】上下文；2) 用户选择的附件或笔记；3) 下方检索到的全库笔记片段。\n")
	sb.WriteString("检索结果只是参考。不要因为检索上下文不足就直接回答'不知道'；只有当用户问题、显式上下文和检索结果都不足时，才说明缺口并给出下一步该补充什么。\n")
	sb.WriteString("如果用户要求整理计划，请直接提炼待办、排序、风险/注意事项、可推迟项，不要输出方法论模板。\n")
	if anchorNoteID > 0 {
		sb.WriteString("当前打开笔记如出现在上下文中会标记为 CURRENT_NOTE；提到当前笔记时以 CURRENT_NOTE 为准。\n")
	}
	sb.WriteString("\n用户请求：\n")
	sb.WriteString(strings.TrimSpace(query))
	sb.WriteString("\n\n可参考的全库笔记片段：\n")
	if len(contexts) == 0 {
		sb.WriteString("无可用检索片段。\n")
	} else {
		for i, c := range contexts {
			role := "RELATED_NOTE"
			if anchorNoteID > 0 && c.NoteID == anchorNoteID {
				role = "CURRENT_NOTE"
			}
			title := strings.TrimSpace(c.NoteTitle)
			titlePart := ""
			if title != "" {
				titlePart = fmt.Sprintf(" title=%q", title)
			}
			sb.WriteString(fmt.Sprintf("[%d] %s note_id=%d%s chunk_index=%d score=%.4f\n%s\n\n",
				i+1, role, c.NoteID, titlePart, c.Index, c.Score, c.Content))
		}
	}
	sb.WriteString("请用中文给出具体、可执行的回答。除非用户要求，否则不要输出内部检索编号。")
	return sb.String()
}
