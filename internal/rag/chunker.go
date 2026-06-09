package rag

import "strings"

func ChunkMarkdown(markdown string, maxChars int) []string {
	if maxChars <= 0 {
		maxChars = 800
	}

	text := strings.ReplaceAll(markdown, "\r\n", "\n")
	paragraphs := splitParagraphs(text)
	if len(paragraphs) == 0 {
		return nil
	}

	var chunks []string
	var current strings.Builder

	flush := func() {
		val := strings.TrimSpace(current.String())
		if val != "" {
			chunks = append(chunks, val)
		}
		current.Reset()
	}

	for _, p := range paragraphs {
		if p == "" {
			continue
		}
		if len(p) > maxChars {
			if current.Len() > 0 {
				flush()
			}
			for _, part := range splitLong(p, maxChars) {
				if strings.TrimSpace(part) != "" {
					chunks = append(chunks, strings.TrimSpace(part))
				}
			}
			continue
		}

		if current.Len() == 0 {
			current.WriteString(p)
			continue
		}

		if current.Len()+2+len(p) <= maxChars {
			current.WriteString("\n\n")
			current.WriteString(p)
			continue
		}

		flush()
		current.WriteString(p)
	}

	if current.Len() > 0 {
		flush()
	}

	return chunks
}

func splitParagraphs(s string) []string {
	var out []string
	var block []string
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(block) > 0 {
				out = append(out, strings.Join(block, "\n"))
				block = block[:0]
			}
			continue
		}
		block = append(block, line)
	}
	if len(block) > 0 {
		out = append(out, strings.Join(block, "\n"))
	}
	return out
}

func splitLong(s string, maxChars int) []string {
	if len(s) <= maxChars {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{s}
	}
	var out []string
	var current strings.Builder
	for _, w := range words {
		if current.Len() == 0 {
			current.WriteString(w)
			continue
		}
		if current.Len()+1+len(w) <= maxChars {
			current.WriteString(" ")
			current.WriteString(w)
			continue
		}
		out = append(out, current.String())
		current.Reset()
		current.WriteString(w)
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}
