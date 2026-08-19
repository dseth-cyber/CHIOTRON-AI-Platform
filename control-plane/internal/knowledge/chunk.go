package knowledge

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// SupportedMimeTypes are the formats the parser understands today.
var SupportedMimeTypes = []string{
	"text/plain",
	"text/markdown",
	"application/json",
	"text/csv",
	"text/tab-separated-values",
}

// ChunkPlan is the chunking policy. Sizes are in characters rather than tokens:
// tokenisation belongs to a model, and the corpus is multilingual, so a
// character budget is the honest approximation at this layer.
type ChunkPlan struct {
	Size    int
	Overlap int
}

func NewChunkPlan(size, overlap int) (ChunkPlan, error) {
	switch {
	case size <= 0:
		return ChunkPlan{}, fmt.Errorf("chunk size must be greater than zero, got %d", size)
	case overlap < 0:
		return ChunkPlan{}, fmt.Errorf("chunk overlap cannot be negative, got %d", overlap)
	case overlap >= size:
		// Otherwise every chunk would repeat the previous one entirely and the
		// loop would never advance.
		return ChunkPlan{}, fmt.Errorf("chunk overlap %d must be smaller than the size %d", overlap, size)
	}
	return ChunkPlan{Size: size, Overlap: overlap}, nil
}

// SupportsMime reports whether the parser can read a content type. The
// parameters after `;` (charset and friends) are ignored.
func SupportsMime(mimeType string) bool {
	base := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	for _, supported := range SupportedMimeTypes {
		if base == supported {
			return true
		}
	}
	return false
}

// Parse turns stored bytes into plain text.
//
// Normalises line endings and rejects content that is not valid UTF-8.
// For JSON, verifies valid JSON structure.
func Parse(mimeType string, content []byte) (string, error) {
	if !SupportsMime(mimeType) {
		return "", fmt.Errorf("unsupported content type %q (supported: %s)",
			mimeType, strings.Join(SupportedMimeTypes, ", "))
	}
	if !utf8.Valid(content) {
		return "", fmt.Errorf("content is not valid UTF-8")
	}

	baseMime := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	if baseMime == "application/json" {
		var raw json.RawMessage
		if err := json.Unmarshal(content, &raw); err != nil {
			return "", fmt.Errorf("invalid JSON document: %w", err)
		}
	}

	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("content is empty")
	}
	return text, nil
}

// Split breaks text into overlapping chunks, preferring paragraph boundaries.
//
// Provenance is the ordinal: a chunk's position in the document is what a
// citation points at, so the order must be stable for the same input.
func (p ChunkPlan) Split(text string) []string {
	paragraphs := splitParagraphs(text)
	if len(paragraphs) == 0 {
		return nil
	}

	var chunks []string
	var current []string
	currentLen := 0

	flush := func() {
		if len(current) == 0 {
			return
		}
		chunks = append(chunks, strings.Join(current, "\n\n"))
		current, currentLen = nil, 0
	}

	for _, paragraph := range paragraphs {
		size := utf8.RuneCountInString(paragraph)

		// A paragraph larger than the budget cannot be kept whole, so it is cut
		// on its own rather than dragging a partial neighbour along.
		if size > p.Size {
			flush()
			chunks = append(chunks, p.hardSplit(paragraph)...)
			continue
		}
		if currentLen > 0 && currentLen+size > p.Size {
			flush()
		}
		current = append(current, paragraph)
		currentLen += size
	}
	flush()

	return p.applyOverlap(chunks)
}

// hardSplit cuts an oversized paragraph on rune boundaries so no chunk contains
// half a character.
func (p ChunkPlan) hardSplit(paragraph string) []string {
	runes := []rune(paragraph)
	step := p.Size - p.Overlap

	var pieces []string
	for start := 0; start < len(runes); start += step {
		end := min(start+p.Size, len(runes))
		piece := strings.TrimSpace(string(runes[start:end]))
		if piece != "" {
			pieces = append(pieces, piece)
		}
		if end == len(runes) {
			break
		}
	}
	return pieces
}

// applyOverlap prefixes each chunk with the tail of the one before it, so a
// sentence split across a boundary is still retrievable from both sides.
func (p ChunkPlan) applyOverlap(chunks []string) []string {
	if p.Overlap == 0 || len(chunks) < 2 {
		return chunks
	}

	overlapped := make([]string, 0, len(chunks))
	overlapped = append(overlapped, chunks[0])
	for i := 1; i < len(chunks); i++ {
		previous := []rune(chunks[i-1])
		tail := previous
		if len(previous) > p.Overlap {
			tail = previous[len(previous)-p.Overlap:]
		}
		overlapped = append(overlapped, strings.TrimSpace(string(tail))+"\n\n"+chunks[i])
	}
	return overlapped
}

func splitParagraphs(text string) []string {
	var paragraphs []string
	for _, block := range strings.Split(text, "\n\n") {
		if trimmed := strings.TrimSpace(block); trimmed != "" {
			paragraphs = append(paragraphs, trimmed)
		}
	}
	return paragraphs
}
