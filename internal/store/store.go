package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("not found")
var ErrInvalidState = errors.New("invalid note state")
var ErrInvalidBlock = errors.New("invalid note block")

type NoteFilter struct {
	Query           string
	Tag             string
	IncludeArchived bool
	OnlyArchived    bool
}

type NoteInput struct {
	ParentID *int64
	Title    string
	Markdown string
	HTML     string
	Tags     []string
	Status   string
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) InitSchema(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id INTEGER NULL REFERENCES notes(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    markdown TEXT NOT NULL,
    html TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unfinished',
    is_pinned INTEGER NOT NULL DEFAULT 0,
    is_archived INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS note_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id INTEGER NOT NULL,
    idx INTEGER NOT NULL,
    content TEXT NOT NULL,
    embedding TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS note_tags (
    note_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    PRIMARY KEY (note_id, tag),
    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS note_blocks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    note_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    level INTEGER NOT NULL DEFAULT 0,
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    checked INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

	CREATE TABLE IF NOT EXISTS note_review_questions (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    note_id INTEGER NOT NULL,
	    question TEXT NOT NULL,
	    answer TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'manual',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS note_insights (
	    note_id INTEGER PRIMARY KEY,
	    content TEXT NOT NULL,
	    note_updated_at DATETIME NOT NULL,
	    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	    FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS knowledge_cards (
	    id INTEGER PRIMARY KEY AUTOINCREMENT,
	    front TEXT NOT NULL,
	    back TEXT NOT NULL,
    tags TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'active',
    review_stage INTEGER NOT NULL DEFAULT 0,
    last_reviewed_at DATETIME NULL,
    next_review_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS research_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic TEXT NOT NULL,
    result TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS recommendation_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    topic TEXT NOT NULL,
    result TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_chunks_note_id ON note_chunks(note_id);
		CREATE INDEX IF NOT EXISTS idx_note_tags_tag ON note_tags(tag);
		CREATE INDEX IF NOT EXISTS idx_note_blocks_note_pos ON note_blocks(note_id, position);
		CREATE INDEX IF NOT EXISTS idx_note_review_questions_note_id ON note_review_questions(note_id, updated_at DESC);
		CREATE INDEX IF NOT EXISTS idx_note_insights_updated ON note_insights(updated_at DESC);
		CREATE INDEX IF NOT EXISTS idx_knowledge_cards_status_due ON knowledge_cards(status, next_review_at);
		CREATE INDEX IF NOT EXISTS idx_research_sessions_created ON research_sessions(created_at DESC, id DESC);
		CREATE INDEX IF NOT EXISTS idx_recommendation_sessions_created ON recommendation_sessions(created_at DESC, id DESC);
		`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return err
	}

	// Backward-compatible migration for existing DB files.
	hasParentID, err := s.hasColumn(ctx, "notes", "parent_id")
	if err != nil {
		return err
	}
	if !hasParentID {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN parent_id INTEGER NULL REFERENCES notes(id) ON DELETE SET NULL`); err != nil {
			return fmt.Errorf("add parent_id column: %w", err)
		}
	}
	hasPinned, err := s.hasColumn(ctx, "notes", "is_pinned")
	if err != nil {
		return err
	}
	if !hasPinned {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN is_pinned INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add is_pinned column: %w", err)
		}
	}
	hasArchived, err := s.hasColumn(ctx, "notes", "is_archived")
	if err != nil {
		return err
	}
	if !hasArchived {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN is_archived INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add is_archived column: %w", err)
		}
	}
	hasStatus, err := s.hasColumn(ctx, "notes", "status")
	if err != nil {
		return err
	}
	if !hasStatus {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN status TEXT NOT NULL DEFAULT 'unfinished'`); err != nil {
			return fmt.Errorf("add status column: %w", err)
		}
	}
	hasBlockLevel, err := s.hasColumn(ctx, "note_blocks", "level")
	if err != nil {
		return err
	}
	if !hasBlockLevel {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE note_blocks ADD COLUMN level INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add note_blocks.level column: %w", err)
		}
	}

	return nil
}

func (s *Store) hasColumn(ctx context.Context, table, column string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			typ       string
			notnull   int
			dfltValue any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) ListNotes(ctx context.Context, filter NoteFilter) ([]Note, error) {
	return s.listNotes(ctx, filter, true)
}

func (s *Store) ListNotesLite(ctx context.Context, filter NoteFilter) ([]Note, error) {
	return s.listNotes(ctx, filter, false)
}

func (s *Store) listNotes(ctx context.Context, filter NoteFilter, withContent bool) ([]Note, error) {
	queryLike := "%" + strings.TrimSpace(filter.Query) + "%"
	tag := normalizeTag(filter.Tag)

	var (
		conds []string
		args  []any
	)
	if strings.TrimSpace(filter.Query) != "" {
		conds = append(conds, "(n.title LIKE ? OR n.markdown LIKE ?)")
		args = append(args, queryLike, queryLike)
	}
	if tag != "" {
		conds = append(conds, "EXISTS (SELECT 1 FROM note_tags t WHERE t.note_id = n.id AND t.tag = ?)")
		args = append(args, tag)
	}
	if filter.OnlyArchived {
		conds = append(conds, "n.is_archived = 1")
	} else if !filter.IncludeArchived {
		conds = append(conds, "n.is_archived = 0")
	}

	selectFields := "n.id, n.parent_id, n.title, n.markdown, n.html, n.status, n.is_pinned, n.is_archived, n.created_at, n.updated_at"
	if !withContent {
		selectFields = "n.id, n.parent_id, n.title, '' AS markdown, '' AS html, n.status, n.is_pinned, n.is_archived, n.created_at, n.updated_at"
	}

	query := `
SELECT ` + selectFields + `
FROM notes n`
	if len(conds) > 0 {
		query += "\nWHERE " + strings.Join(conds, " AND ")
	}
	query += "\nORDER BY n.is_pinned DESC, n.updated_at DESC, n.id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Note
	for rows.Next() {
		var (
			n        Note
			parentID sql.NullInt64
			pinned   int
			archived int
		)
		if err := rows.Scan(&n.ID, &parentID, &n.Title, &n.Markdown, &n.HTML, &n.Status, &pinned, &archived, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		if parentID.Valid {
			n.ParentID = &parentID.Int64
		}
		n.IsPinned = pinned == 1
		n.IsArchived = archived == 1
		n.Status = normalizeNoteStatus(n.Status)
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(out))
	for _, n := range out {
		ids = append(ids, n.ID)
	}
	tagsMap, err := s.tagsByNoteIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Tags = tagsMap[out[i].ID]
	}

	return out, nil
}

func (s *Store) ListDistinctTags(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT tag
FROM note_tags
ORDER BY tag ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *Store) DeleteTag(ctx context.Context, tag string) error {
	clean := strings.TrimSpace(tag)
	if clean == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
DELETE FROM note_tags
WHERE tag = ? COLLATE NOCASE`, clean)
	return err
}

func (s *Store) GetNote(ctx context.Context, id int64) (Note, error) {
	var (
		n        Note
		parentID sql.NullInt64
		pinned   int
		archived int
	)
	err := s.db.QueryRowContext(ctx, `
SELECT id, parent_id, title, markdown, html, status, is_pinned, is_archived, created_at, updated_at
FROM notes
WHERE id = ?`, id).Scan(&n.ID, &parentID, &n.Title, &n.Markdown, &n.HTML, &n.Status, &pinned, &archived, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Note{}, ErrNotFound
	}
	if err != nil {
		return Note{}, err
	}
	if parentID.Valid {
		n.ParentID = &parentID.Int64
	}
	n.IsPinned = pinned == 1
	n.IsArchived = archived == 1
	n.Status = normalizeNoteStatus(n.Status)
	tagsMap, err := s.tagsByNoteIDs(ctx, []int64{id})
	if err != nil {
		return Note{}, err
	}
	n.Tags = tagsMap[id]
	return n, nil
}

func (s *Store) CreateNote(ctx context.Context, in NoteInput) (Note, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Note{}, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
INSERT INTO notes (parent_id, title, markdown, html, status, is_pinned, is_archived)
VALUES (?, ?, ?, ?, ?, 0, 0)`, in.ParentID, in.Title, in.Markdown, in.HTML, normalizeNoteStatus(in.Status))
	if err != nil {
		return Note{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Note{}, err
	}

	if err := replaceTagsTx(ctx, tx, id, in.Tags); err != nil {
		return Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return Note{}, err
	}
	return s.GetNote(ctx, id)
}

func (s *Store) UpdateNote(ctx context.Context, id int64, in NoteInput) (Note, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Note{}, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
UPDATE notes
SET parent_id = ?, title = ?, markdown = ?, html = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, in.ParentID, in.Title, in.Markdown, in.HTML, id)
	if err != nil {
		return Note{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Note{}, err
	}
	if affected == 0 {
		return Note{}, ErrNotFound
	}

	if err := replaceTagsTx(ctx, tx, id, in.Tags); err != nil {
		return Note{}, err
	}
	if err := tx.Commit(); err != nil {
		return Note{}, err
	}
	return s.GetNote(ctx, id)
}

func (s *Store) DeleteNote(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}

	// Move children to root when parent is deleted.
	_, _ = s.db.ExecContext(ctx, `UPDATE notes SET parent_id = NULL WHERE parent_id = ?`, id)
	return nil
}

func (s *Store) SetPinned(ctx context.Context, id int64, pinned bool) (Note, error) {
	n, err := s.GetNote(ctx, id)
	if err != nil {
		return Note{}, err
	}
	if n.IsArchived && pinned {
		return Note{}, ErrInvalidState
	}

	v := 0
	if pinned {
		v = 1
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE notes
SET is_pinned = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, v, id)
	if err != nil {
		return Note{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Note{}, err
	}
	if affected == 0 {
		return Note{}, ErrNotFound
	}
	return s.GetNote(ctx, id)
}

func (s *Store) SetArchived(ctx context.Context, id int64, archived bool) (Note, error) {
	v := 0
	if archived {
		v = 1
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE notes
SET is_archived = ?,
    is_pinned = CASE WHEN ? = 1 THEN 0 ELSE is_pinned END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, v, v, id)
	if err != nil {
		return Note{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Note{}, err
	}
	if affected == 0 {
		return Note{}, ErrNotFound
	}
	return s.GetNote(ctx, id)
}

func (s *Store) ListNoteBlocks(ctx context.Context, noteID int64) ([]NoteBlock, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, note_id, position, level, type, content, checked, created_at, updated_at
FROM note_blocks
WHERE note_id = ?
ORDER BY position ASC, id ASC`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NoteBlock
	for rows.Next() {
		var (
			b       NoteBlock
			checked int
		)
		if err := rows.Scan(&b.ID, &b.NoteID, &b.Position, &b.Level, &b.Type, &b.Content, &checked, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.Level = normalizeBlockLevel(b.Level)
		b.Checked = checked == 1
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) ReplaceNoteBlocks(ctx context.Context, noteID int64, blocks []NoteBlockInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM note_blocks WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	if len(blocks) == 0 {
		return tx.Commit()
	}

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO note_blocks (note_id, position, level, type, content, checked)
VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, b := range blocks {
		typ := normalizeBlockType(b.Type)
		if typ == "" {
			return ErrInvalidBlock
		}
		content := strings.TrimSpace(strings.ReplaceAll(b.Content, "\r\n", "\n"))
		if content == "" {
			continue
		}
		checked := 0
		if b.Checked {
			checked = 1
		}
		level := normalizeBlockLevel(b.Level)
		if _, err := stmt.ExecContext(ctx, noteID, i, level, typ, content, checked); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListReviewQuestions(ctx context.Context, noteID int64) ([]ReviewQuestion, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, note_id, question, answer, source, created_at, updated_at
FROM note_review_questions
WHERE note_id = ?
ORDER BY updated_at DESC, id DESC`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ReviewQuestion
	for rows.Next() {
		var q ReviewQuestion
		if err := rows.Scan(&q.ID, &q.NoteID, &q.Question, &q.Answer, &q.Source, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *Store) CreateReviewQuestion(ctx context.Context, noteID int64, in ReviewQuestionInput) (ReviewQuestion, error) {
	if _, err := s.GetNote(ctx, noteID); err != nil {
		return ReviewQuestion{}, err
	}
	question := strings.TrimSpace(in.Question)
	if question == "" {
		return ReviewQuestion{}, ErrInvalidState
	}
	answer := strings.TrimSpace(in.Answer)
	source := normalizeReviewQuestionSource(in.Source)

	res, err := s.db.ExecContext(ctx, `
INSERT INTO note_review_questions (note_id, question, answer, source)
VALUES (?, ?, ?, ?)`, noteID, question, answer, source)
	if err != nil {
		return ReviewQuestion{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ReviewQuestion{}, err
	}
	return s.GetReviewQuestion(ctx, noteID, id)
}

func (s *Store) GetReviewQuestion(ctx context.Context, noteID, id int64) (ReviewQuestion, error) {
	var q ReviewQuestion
	err := s.db.QueryRowContext(ctx, `
SELECT id, note_id, question, answer, source, created_at, updated_at
FROM note_review_questions
WHERE note_id = ? AND id = ?`, noteID, id).Scan(&q.ID, &q.NoteID, &q.Question, &q.Answer, &q.Source, &q.CreatedAt, &q.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ReviewQuestion{}, ErrNotFound
	}
	if err != nil {
		return ReviewQuestion{}, err
	}
	return q, nil
}

func (s *Store) UpdateReviewQuestion(ctx context.Context, noteID, id int64, in ReviewQuestionInput) (ReviewQuestion, error) {
	question := strings.TrimSpace(in.Question)
	if question == "" {
		return ReviewQuestion{}, ErrInvalidState
	}
	answer := strings.TrimSpace(in.Answer)

	res, err := s.db.ExecContext(ctx, `
UPDATE note_review_questions
SET question = ?, answer = ?, updated_at = CURRENT_TIMESTAMP
WHERE note_id = ? AND id = ?`, question, answer, noteID, id)
	if err != nil {
		return ReviewQuestion{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return ReviewQuestion{}, err
	}
	if affected == 0 {
		return ReviewQuestion{}, ErrNotFound
	}
	return s.GetReviewQuestion(ctx, noteID, id)
}

func (s *Store) DeleteReviewQuestion(ctx context.Context, noteID, id int64) error {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM note_review_questions
WHERE note_id = ? AND id = ?`, noteID, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetNoteInsight(ctx context.Context, noteID int64) (NoteInsight, error) {
	var insight NoteInsight
	row := s.db.QueryRowContext(ctx, `
SELECT note_id, content, note_updated_at, created_at, updated_at
FROM note_insights
WHERE note_id = ?`, noteID)
	if err := row.Scan(&insight.NoteID, &insight.Content, &insight.NoteUpdatedAt, &insight.CreatedAt, &insight.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return NoteInsight{}, ErrNotFound
	} else if err != nil {
		return NoteInsight{}, err
	}
	return insight, nil
}

func (s *Store) SaveNoteInsight(ctx context.Context, noteID int64, noteUpdatedAt time.Time, content []byte) error {
	if noteID <= 0 || len(content) == 0 {
		return ErrInvalidState
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO note_insights (note_id, content, note_updated_at)
VALUES (?, ?, ?)
ON CONFLICT(note_id) DO UPDATE SET
    content = excluded.content,
    note_updated_at = excluded.note_updated_at,
    updated_at = CURRENT_TIMESTAMP`, noteID, string(content), noteUpdatedAt)
	return err
}

func (s *Store) SetNoteStatus(ctx context.Context, id int64, status string) (Note, error) {
	clean := normalizeNoteStatus(status)
	res, err := s.db.ExecContext(ctx, `
UPDATE notes
SET status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, clean, id)
	if err != nil {
		return Note{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return Note{}, err
	}
	if affected == 0 {
		return Note{}, ErrNotFound
	}
	return s.GetNote(ctx, id)
}

func (s *Store) ListKnowledgeCards(ctx context.Context, filter CardFilter) ([]KnowledgeCard, error) {
	var conds []string
	var args []any
	q := strings.TrimSpace(filter.Query)
	if q != "" {
		like := "%" + q + "%"
		conds = append(conds, "(front LIKE ? OR back LIKE ? OR tags LIKE ?)")
		args = append(args, like, like, like)
	}
	status := normalizeCardStatus(filter.Status)
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	} else if !filter.IncludeArchived {
		conds = append(conds, "status != 'archived'")
	}
	if filter.DueOnly {
		conds = append(conds, "status = 'active' AND (next_review_at IS NULL OR next_review_at <= CURRENT_TIMESTAMP)")
	}

	query := `
SELECT id, front, back, tags, status, review_stage, last_reviewed_at, next_review_at, created_at, updated_at
FROM knowledge_cards`
	if len(conds) > 0 {
		query += "\nWHERE " + strings.Join(conds, " AND ")
	}
	query += "\nORDER BY CASE WHEN next_review_at IS NULL THEN 0 ELSE 1 END ASC, next_review_at ASC, updated_at DESC, id DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []KnowledgeCard
	for rows.Next() {
		card, err := scanKnowledgeCard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, card)
	}
	return out, rows.Err()
}

func (s *Store) GetKnowledgeCard(ctx context.Context, id int64) (KnowledgeCard, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, front, back, tags, status, review_stage, last_reviewed_at, next_review_at, created_at, updated_at
FROM knowledge_cards
WHERE id = ?`, id)
	card, err := scanKnowledgeCard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return KnowledgeCard{}, ErrNotFound
	}
	if err != nil {
		return KnowledgeCard{}, err
	}
	return card, nil
}

func (s *Store) CreateKnowledgeCard(ctx context.Context, in KnowledgeCardInput) (KnowledgeCard, error) {
	front := strings.TrimSpace(in.Front)
	back := strings.TrimSpace(in.Back)
	if front == "" || back == "" {
		return KnowledgeCard{}, ErrInvalidState
	}
	tagsRaw, err := json.Marshal(normalizeTags(in.Tags))
	if err != nil {
		return KnowledgeCard{}, err
	}
	status := normalizeCardStatus(in.Status)
	if status == "" {
		status = "active"
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO knowledge_cards (front, back, tags, status, review_stage, next_review_at)
VALUES (?, ?, ?, ?, 0, CURRENT_TIMESTAMP)`, front, back, string(tagsRaw), status)
	if err != nil {
		return KnowledgeCard{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return KnowledgeCard{}, err
	}
	return s.GetKnowledgeCard(ctx, id)
}

func (s *Store) UpdateKnowledgeCard(ctx context.Context, id int64, in KnowledgeCardInput) (KnowledgeCard, error) {
	front := strings.TrimSpace(in.Front)
	back := strings.TrimSpace(in.Back)
	if front == "" || back == "" {
		return KnowledgeCard{}, ErrInvalidState
	}
	tagsRaw, err := json.Marshal(normalizeTags(in.Tags))
	if err != nil {
		return KnowledgeCard{}, err
	}
	status := normalizeCardStatus(in.Status)
	if status == "" {
		status = "active"
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE knowledge_cards
SET front = ?, back = ?, tags = ?, status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, front, back, string(tagsRaw), status, id)
	if err != nil {
		return KnowledgeCard{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return KnowledgeCard{}, err
	}
	if affected == 0 {
		return KnowledgeCard{}, ErrNotFound
	}
	return s.GetKnowledgeCard(ctx, id)
}

func (s *Store) DeleteKnowledgeCard(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM knowledge_cards WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ReviewKnowledgeCard(ctx context.Context, id int64, remembered bool, now time.Time) (KnowledgeCard, error) {
	card, err := s.GetKnowledgeCard(ctx, id)
	if err != nil {
		return KnowledgeCard{}, err
	}
	stage := card.ReviewStage
	status := "active"
	if remembered {
		stage = card.ReviewStage + 1
		if stage >= len(ebbinghausIntervals) {
			stage = len(ebbinghausIntervals)
			status = "mastered"
		}
	}
	next := now
	if remembered && stage > 0 && stage <= len(ebbinghausIntervals) {
		next = now.Add(time.Duration(ebbinghausIntervals[stage-1]) * 24 * time.Hour)
	}
	res, err := s.db.ExecContext(ctx, `
UPDATE knowledge_cards
SET status = ?, review_stage = ?, last_reviewed_at = ?, next_review_at = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`, status, stage, now, next, id)
	if err != nil {
		return KnowledgeCard{}, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return KnowledgeCard{}, err
	}
	if affected == 0 {
		return KnowledgeCard{}, ErrNotFound
	}
	return s.GetKnowledgeCard(ctx, id)
}

func (s *Store) CreateResearchSession(ctx context.Context, topic string, result []byte) (ResearchSession, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" || len(result) == 0 {
		return ResearchSession{}, ErrInvalidState
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ResearchSession{}, err
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRowContext(ctx, `
SELECT id
FROM research_sessions
WHERE lower(topic) = lower(?)
ORDER BY created_at DESC, id DESC
LIMIT 1`, topic).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ResearchSession{}, err
	}
	if existingID > 0 {
		if _, err := tx.ExecContext(ctx, `
UPDATE research_sessions
SET topic = ?, result = ?, created_at = CURRENT_TIMESTAMP
WHERE id = ?`, topic, string(result), existingID); err != nil {
			return ResearchSession{}, err
		}
		if _, err := tx.ExecContext(ctx, `
DELETE FROM research_sessions
WHERE lower(topic) = lower(?) AND id != ?`, topic, existingID); err != nil {
			return ResearchSession{}, err
		}
		if err := tx.Commit(); err != nil {
			return ResearchSession{}, err
		}
		return s.GetResearchSession(ctx, existingID)
	}

	res, err := tx.ExecContext(ctx, `
INSERT INTO research_sessions (topic, result)
VALUES (?, ?)`, topic, string(result))
	if err != nil {
		return ResearchSession{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ResearchSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return ResearchSession{}, err
	}
	return s.GetResearchSession(ctx, id)
}

func (s *Store) GetResearchSession(ctx context.Context, id int64) (ResearchSession, error) {
	var session ResearchSession
	err := s.db.QueryRowContext(ctx, `
SELECT id, topic, result, created_at
FROM research_sessions
WHERE id = ?`, id).Scan(&session.ID, &session.Topic, &session.Result, &session.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ResearchSession{}, ErrNotFound
	}
	if err != nil {
		return ResearchSession{}, err
	}
	return session, nil
}

func (s *Store) ListResearchSessions(ctx context.Context, limit int) ([]ResearchSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, topic, result, created_at
FROM research_sessions
ORDER BY created_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ResearchSession
	for rows.Next() {
		var session ResearchSession
		if err := rows.Scan(&session.ID, &session.Topic, &session.Result, &session.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	return out, rows.Err()
}

func (s *Store) DeleteResearchSession(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM research_sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateRecommendationSession(ctx context.Context, topic string, result []byte) (RecommendationSession, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" || len(result) == 0 {
		return RecommendationSession{}, ErrInvalidState
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RecommendationSession{}, err
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRowContext(ctx, `
SELECT id
FROM recommendation_sessions
WHERE lower(topic) = lower(?)
ORDER BY created_at DESC, id DESC
LIMIT 1`, topic).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return RecommendationSession{}, err
	}
	if existingID > 0 {
		if _, err := tx.ExecContext(ctx, `
UPDATE recommendation_sessions
SET topic = ?, result = ?, created_at = CURRENT_TIMESTAMP
WHERE id = ?`, topic, string(result), existingID); err != nil {
			return RecommendationSession{}, err
		}
		if _, err := tx.ExecContext(ctx, `
DELETE FROM recommendation_sessions
WHERE lower(topic) = lower(?) AND id != ?`, topic, existingID); err != nil {
			return RecommendationSession{}, err
		}
		if err := tx.Commit(); err != nil {
			return RecommendationSession{}, err
		}
		return s.GetRecommendationSession(ctx, existingID)
	}

	res, err := tx.ExecContext(ctx, `
INSERT INTO recommendation_sessions (topic, result)
VALUES (?, ?)`, topic, string(result))
	if err != nil {
		return RecommendationSession{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return RecommendationSession{}, err
	}
	if err := tx.Commit(); err != nil {
		return RecommendationSession{}, err
	}
	return s.GetRecommendationSession(ctx, id)
}

func (s *Store) GetRecommendationSession(ctx context.Context, id int64) (RecommendationSession, error) {
	var session RecommendationSession
	err := s.db.QueryRowContext(ctx, `
SELECT id, topic, result, created_at
FROM recommendation_sessions
WHERE id = ?`, id).Scan(&session.ID, &session.Topic, &session.Result, &session.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RecommendationSession{}, ErrNotFound
	}
	if err != nil {
		return RecommendationSession{}, err
	}
	return session, nil
}

func (s *Store) ListRecommendationSessions(ctx context.Context, limit int) ([]RecommendationSession, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, topic, result, created_at
FROM recommendation_sessions
ORDER BY created_at DESC, id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RecommendationSession
	for rows.Next() {
		var session RecommendationSession
		if err := rows.Scan(&session.ID, &session.Topic, &session.Result, &session.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, session)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRecommendationSession(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM recommendation_sessions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

type knowledgeCardScanner interface {
	Scan(dest ...any) error
}

var ebbinghausIntervals = []int{1, 2, 4, 7, 15, 30}

func scanKnowledgeCard(row knowledgeCardScanner) (KnowledgeCard, error) {
	var (
		card           KnowledgeCard
		rawTags        string
		lastReviewedAt sql.NullTime
		nextReviewAt   sql.NullTime
	)
	if err := row.Scan(&card.ID, &card.Front, &card.Back, &rawTags, &card.Status, &card.ReviewStage, &lastReviewedAt, &nextReviewAt, &card.CreatedAt, &card.UpdatedAt); err != nil {
		return KnowledgeCard{}, err
	}
	card.Status = normalizeCardStatus(card.Status)
	if card.Status == "" {
		card.Status = "active"
	}
	if lastReviewedAt.Valid {
		card.LastReviewedAt = &lastReviewedAt.Time
	}
	if nextReviewAt.Valid {
		card.NextReviewAt = &nextReviewAt.Time
	}
	_ = json.Unmarshal([]byte(rawTags), &card.Tags)
	card.Tags = normalizeTags(card.Tags)
	return card, nil
}

func replaceTagsTx(ctx context.Context, tx *sql.Tx, noteID int64, tags []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM note_tags WHERE note_id = ?`, noteID); err != nil {
		return err
	}

	tags = normalizeTags(tags)
	if len(tags) == 0 {
		return nil
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO note_tags (note_id, tag) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, tag := range tags {
		if _, err := stmt.ExecContext(ctx, noteID, tag); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		t := normalizeTag(tag)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

func normalizeNoteStatus(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "completed":
		return "completed"
	default:
		return "unfinished"
	}
}

func normalizeCardStatus(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "active", "archived", "mastered":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return ""
	}
}

func normalizeBlockType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "paragraph", "todo", "heading1", "code", "quote", "table":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return ""
	}
}

func normalizeBlockLevel(v int) int {
	if v < 0 {
		return 0
	}
	if v > 6 {
		return 6
	}
	return v
}

func normalizeReviewQuestionSource(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ai":
		return "ai"
	default:
		return "manual"
	}
}

func (s *Store) tagsByNoteIDs(ctx context.Context, idsInput []int64) (map[int64][]string, error) {
	out := make(map[int64][]string)
	if len(idsInput) == 0 {
		return out, nil
	}

	ids := make([]string, 0, len(idsInput))
	args := make([]any, 0, len(idsInput))
	for _, id := range idsInput {
		ids = append(ids, "?")
		args = append(args, id)
	}

	query := fmt.Sprintf(`
SELECT note_id, tag
FROM note_tags
WHERE note_id IN (%s)
ORDER BY tag ASC`, strings.Join(ids, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			noteID int64
			tag    string
		)
		if err := rows.Scan(&noteID, &tag); err != nil {
			return nil, err
		}
		out[noteID] = append(out[noteID], tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ReplaceNoteChunks(ctx context.Context, noteID int64, chunks []Chunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM note_chunks WHERE note_id = ?`, noteID); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO note_chunks (note_id, idx, content, embedding)
VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range chunks {
		raw, err := json.Marshal(c.Embedding)
		if err != nil {
			return fmt.Errorf("marshal embedding: %w", err)
		}
		if _, err := stmt.ExecContext(ctx, noteID, c.Idx, c.Content, string(raw)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListChunks(ctx context.Context) ([]Chunk, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, note_id, idx, content, embedding, created_at
FROM note_chunks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Chunk
	for rows.Next() {
		var (
			c       Chunk
			rawJSON string
		)
		if err := rows.Scan(&c.ID, &c.NoteID, &c.Idx, &c.Content, &rawJSON, &c.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(rawJSON), &c.Embedding); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) NowUTC() time.Time {
	return time.Now().UTC()
}
