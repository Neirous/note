package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ---- Notes CRUD ----

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

	_, _ = s.db.ExecContext(ctx, `UPDATE notes SET parent_id = NULL WHERE parent_id = ?`, id)
	return nil
}

// ---- Tags ----

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

// ---- Pin / Archive / Status ----

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

// ---- Blocks ----

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
