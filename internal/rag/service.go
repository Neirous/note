package rag

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"note/internal/store"
)

type ChunkWithScore struct {
	NoteID    int64   `json:"note_id"`
	NoteTitle string  `json:"note_title,omitempty"`
	ChunkID   int64   `json:"chunk_id"`
	Index     int     `json:"index"`
	Content   string  `json:"content"`
	Score     float64 `json:"score"`
}

type SearchResult struct {
	Query   string           `json:"query"`
	Results []ChunkWithScore `json:"results"`
}

type AskResult struct {
	Query    string           `json:"query"`
	Answer   string           `json:"answer"`
	Contexts []ChunkWithScore `json:"contexts"`
}

type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float64, error)
}

type Generator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

type Config struct {
	MaxChunkChars int
	TopK          int
}

type SearchOptions struct {
	AnchorNoteID     int64
	PrioritizeAnchor bool
}

type Service struct {
	store     *store.Store
	embedder  EmbeddingProvider
	generator Generator
	cfg       Config
}

func NewService(store *store.Store, embedder EmbeddingProvider, generator Generator, cfg Config) *Service {
	if cfg.MaxChunkChars <= 0 {
		cfg.MaxChunkChars = 800
	}
	if cfg.TopK <= 0 {
		cfg.TopK = 5
	}
	return &Service{
		store:     store,
		embedder:  embedder,
		generator: generator,
		cfg:       cfg,
	}
}

func (s *Service) IndexNote(ctx context.Context, noteID int64, markdown string) error {
	if s.embedder == nil {
		return errors.New("embedder not configured")
	}

	chunkTexts := ChunkMarkdown(markdown, s.cfg.MaxChunkChars)
	if len(chunkTexts) == 0 {
		return s.store.ReplaceNoteChunks(ctx, noteID, nil)
	}

	chunks := make([]store.Chunk, 0, len(chunkTexts))
	for i, text := range chunkTexts {
		embedding, err := s.embedder.Embed(ctx, text)
		if err != nil {
			return fmt.Errorf("embed chunk %d: %w", i, err)
		}
		chunks = append(chunks, store.Chunk{
			Idx:       i,
			Content:   text,
			Embedding: embedding,
		})
	}
	return s.store.ReplaceNoteChunks(ctx, noteID, chunks)
}

func (s *Service) Search(ctx context.Context, query string, topK int) (SearchResult, error) {
	return s.SearchWithOptions(ctx, query, topK, SearchOptions{})
}

func (s *Service) SearchWithOptions(ctx context.Context, query string, topK int, opts SearchOptions) (SearchResult, error) {
	if s.embedder == nil {
		return SearchResult{}, errors.New("embedder not configured")
	}
	if strings.TrimSpace(query) == "" {
		return SearchResult{}, errors.New("query is empty")
	}

	qEmbedding, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return SearchResult{}, fmt.Errorf("embed query: %w", err)
	}

	notes, err := s.store.ListNotesLite(ctx, store.NoteFilter{})
	if err != nil {
		return SearchResult{}, err
	}
	noteByID := make(map[int64]store.Note, len(notes))
	for _, note := range notes {
		noteByID[note.ID] = note
	}
	anchor, hasAnchor := noteByID[opts.AnchorNoteID]

	chunks, err := s.store.ListChunks(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	if hasAnchor && !chunksContainNote(chunks, opts.AnchorNoteID) {
		anchorNote, err := s.store.GetNote(ctx, opts.AnchorNoteID)
		if err != nil {
			return SearchResult{}, err
		}
		noteByID[anchorNote.ID] = anchorNote
		fallbackChunks, err := s.ephemeralChunks(ctx, anchorNote)
		if err != nil {
			return SearchResult{}, err
		}
		chunks = append(chunks, fallbackChunks...)
		anchor = anchorNote
		hasAnchor = true
	}

	queryText := strings.TrimSpace(query)
	queryTokens := tokenizeQuery(queryText)
	scored := make([]ChunkWithScore, 0, len(chunks))
	for _, c := range chunks {
		note, ok := noteByID[c.NoteID]
		if !ok {
			continue
		}
		score := cosineSimilarity(qEmbedding, c.Embedding)
		score = score*0.68 +
			lexicalMatchScore(queryText, queryTokens, c.Content)*0.22 +
			noteMetadataBoost(queryText, queryTokens, note) +
			anchorNoteBoost(note, anchor, noteByID, hasAnchor)
		scored = append(scored, ChunkWithScore{
			NoteID:    c.NoteID,
			NoteTitle: note.Title,
			ChunkID:   c.ID,
			Index:     c.Idx,
			Content:   c.Content,
			Score:     score,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			if scored[i].NoteID == scored[j].NoteID {
				return scored[i].Index < scored[j].Index
			}
			return scored[i].NoteID < scored[j].NoteID
		}
		return scored[i].Score > scored[j].Score
	})
	if opts.PrioritizeAnchor && hasAnchor {
		scored = prioritizeAnchorResults(scored, opts.AnchorNoteID)
	}

	if topK <= 0 {
		topK = s.cfg.TopK
	}
	if topK > len(scored) {
		topK = len(scored)
	}

	return SearchResult{
		Query:   query,
		Results: scored[:topK],
	}, nil
}

func (s *Service) Ask(ctx context.Context, query string, topK int) (AskResult, error) {
	return s.AskWithOptions(ctx, query, topK, SearchOptions{})
}

func (s *Service) AskWithOptions(ctx context.Context, query string, topK int, opts SearchOptions) (AskResult, error) {
	if s.generator == nil {
		return AskResult{}, errors.New("generator not configured")
	}
	askOpts := opts
	if askOpts.AnchorNoteID > 0 {
		askOpts.PrioritizeAnchor = true
	}
	searchResult, err := s.SearchWithOptions(ctx, query, topK, askOpts)
	if err != nil {
		return AskResult{}, err
	}
	prompt := buildPrompt(query, searchResult.Results, askOpts.AnchorNoteID)
	answer, err := s.generator.Generate(ctx, prompt)
	if err != nil {
		return AskResult{}, err
	}

	return AskResult{
		Query:    query,
		Answer:   answer,
		Contexts: searchResult.Results,
	}, nil
}

func (s *Service) Generate(ctx context.Context, prompt string) (string, error) {
	if s.generator == nil {
		return "", errors.New("generator not configured")
	}
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("prompt is empty")
	}
	return s.generator.Generate(ctx, prompt)
}

func buildPrompt(query string, contexts []ChunkWithScore, anchorNoteID int64) string {
	var sb strings.Builder
	sb.WriteString("你是笔记问答助手。必须优先使用给定上下文回答，不知道就明确说不知道。\n")
	if anchorNoteID > 0 {
		sb.WriteString("当前打开笔记已在上下文中用 CURRENT_NOTE 标记；当问题提到“当前笔记”“这篇笔记”“这页”或“这里”时，必须以 CURRENT_NOTE 为准，不要把 RELATED_NOTE 当作当前笔记。\n")
	}
	sb.WriteString("问题：")
	sb.WriteString(query)
	sb.WriteString("\n\n上下文：\n")
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
	sb.WriteString("请基于上下文给出简洁答案，不要在回答中输出类似[1][2]的编号。")
	return sb.String()
}

func prioritizeAnchorResults(results []ChunkWithScore, anchorNoteID int64) []ChunkWithScore {
	if anchorNoteID <= 0 || len(results) == 0 {
		return results
	}
	out := make([]ChunkWithScore, 0, len(results))
	for _, r := range results {
		if r.NoteID == anchorNoteID {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return results
	}
	for _, r := range results {
		if r.NoteID != anchorNoteID {
			out = append(out, r)
		}
	}
	return out
}

func chunksContainNote(chunks []store.Chunk, noteID int64) bool {
	for _, c := range chunks {
		if c.NoteID == noteID {
			return true
		}
	}
	return false
}

func (s *Service) ephemeralChunks(ctx context.Context, note store.Note) ([]store.Chunk, error) {
	chunkTexts := ChunkMarkdown(note.Markdown, s.cfg.MaxChunkChars)
	chunks := make([]store.Chunk, 0, len(chunkTexts))
	for i, text := range chunkTexts {
		embedding, err := s.embedder.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("embed anchor fallback chunk %d: %w", i, err)
		}
		chunks = append(chunks, store.Chunk{
			NoteID:    note.ID,
			Idx:       i,
			Content:   text,
			Embedding: embedding,
		})
	}
	return chunks, nil
}

var queryTokenRE = regexp.MustCompile(`[\p{Han}]{2,}|[a-z0-9_+\-.]+`)

func tokenizeQuery(query string) []string {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, 8)
	add := func(token string) {
		token = strings.TrimSpace(token)
		if token == "" {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}

	for _, token := range queryTokenRE.FindAllString(lower, -1) {
		add(token)
		runes := []rune(token)
		if len(runes) >= 3 {
			for i := 0; i < len(runes)-1; i++ {
				add(string(runes[i : i+2]))
			}
		}
	}
	add(lower)
	return out
}

func lexicalMatchScore(query string, tokens []string, text string) float64 {
	lowerText := strings.ToLower(strings.TrimSpace(text))
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerText == "" || lowerQuery == "" {
		return 0
	}

	score := 0.0
	if strings.Contains(lowerText, lowerQuery) {
		score += 0.22
	}
	for _, token := range tokens {
		if token == "" {
			continue
		}
		count := strings.Count(lowerText, token)
		if count == 0 {
			continue
		}
		score += math.Min(float64(count), 3) * 0.05
	}
	return math.Min(score, 0.48)
}

func noteMetadataBoost(query string, tokens []string, note store.Note) float64 {
	title := strings.ToLower(strings.TrimSpace(note.Title))
	if title == "" && len(note.Tags) == 0 {
		return 0
	}

	score := 0.0
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	if lowerQuery != "" && title != "" && strings.Contains(title, lowerQuery) {
		score += 0.22
	}
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if title != "" && strings.Contains(title, token) {
			score += 0.08
		}
		for _, tag := range note.Tags {
			if strings.Contains(strings.ToLower(tag), token) {
				score += 0.07
			}
		}
	}
	return math.Min(score, 0.34)
}

func anchorNoteBoost(note, anchor store.Note, byID map[int64]store.Note, hasAnchor bool) float64 {
	if !hasAnchor {
		return 0
	}
	if note.ID == anchor.ID {
		return 0.34
	}

	score := 0.0
	if sharesFolderContext(note, anchor, byID) {
		score += 0.15
	}
	score += math.Min(float64(sharedTagCount(note, anchor))*0.06, 0.18)
	return score
}

func sharedTagCount(a, b store.Note) int {
	if len(a.Tags) == 0 || len(b.Tags) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(a.Tags))
	for _, tag := range a.Tags {
		set[strings.ToLower(strings.TrimSpace(tag))] = struct{}{}
	}
	count := 0
	for _, tag := range b.Tags {
		if _, ok := set[strings.ToLower(strings.TrimSpace(tag))]; ok {
			count++
		}
	}
	return count
}

func sharesFolderContext(a, b store.Note, byID map[int64]store.Note) bool {
	if a.ID == b.ID {
		return true
	}
	ancestors := ancestorSet(a, byID)
	if _, ok := ancestors[b.ID]; ok {
		return true
	}
	for id := range ancestorSet(b, byID) {
		if _, ok := ancestors[id]; ok {
			return true
		}
	}
	return false
}

func ancestorSet(note store.Note, byID map[int64]store.Note) map[int64]struct{} {
	out := map[int64]struct{}{note.ID: struct{}{}}
	cur := note
	for cur.ParentID != nil {
		parent, ok := byID[*cur.ParentID]
		if !ok {
			break
		}
		out[parent.ID] = struct{}{}
		cur = parent
	}
	return out
}
