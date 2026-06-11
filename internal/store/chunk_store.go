package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// ReplaceNoteChunks replaces all chunks for a note in a single transaction.
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

// ListChunks returns all chunks across all notes.
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
