package rag

import "testing"

func TestChunkMarkdown(t *testing.T) {
	in := "# Title\n\nParagraph one line.\n\nParagraph two with more words.\n\nParagraph three."
	chunks := ChunkMarkdown(in, 40)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(c) == 0 {
			t.Fatal("chunk should not be empty")
		}
		if len(c) > 40 {
			t.Fatalf("chunk exceeded max length: %d", len(c))
		}
	}
}
