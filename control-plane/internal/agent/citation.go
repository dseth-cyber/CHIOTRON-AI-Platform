package agent

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/chiotron/ai-control-plane/internal/knowledge"
)

// citationPattern matches the markers the synthesis prompt asks for.
var citationPattern = regexp.MustCompile(`\[(\d{1,3})\]`)

// Citation is one passage offered to the model, numbered so an answer can point
// at it.
type Citation struct {
	Index          int     `json:"index"`
	DocumentID     string  `json:"documentId"`
	DocumentTitle  string  `json:"documentTitle"`
	ChunkOrdinal   int     `json:"chunkOrdinal"`
	Classification string  `json:"classification"`
	Score          float64 `json:"score"`
	// VectorScore is cosine similarity, which unlike the fused score discriminates
	// between "both ranked highly" and "both are about the same thing".
	VectorScore float64 `json:"vectorScore"`
	Used        bool    `json:"used"`

	// chunkID ties a citation back to the passage it came from. It stays
	// unexported: it is an internal row id, not something a client should key on.
	chunkID int64
}

// Citations numbers hits by rank and drops repeats.
//
// Multi-round retrieval returns overlapping chunks by design, and offering the
// same passage twice would let the model cite two numbers for one source.
//
// The result is sorted by score, which the callers below depend on: one round
// arrives already ranked, but two rounds concatenated do not, and reading the
// first entry as the best would let a weak early hit hide a strong later one.
func Citations(hits []knowledge.Hit) []Citation {
	seen := make(map[int64]bool, len(hits))
	citations := make([]Citation, 0, len(hits))
	for _, hit := range hits {
		if seen[hit.ChunkID] {
			continue
		}
		seen[hit.ChunkID] = true
		citations = append(citations, Citation{
			DocumentID:     hit.DocumentID,
			DocumentTitle:  hit.DocumentTitle,
			ChunkOrdinal:   hit.Ordinal,
			Classification: hit.Classification,
			Score:          hit.Score,
			VectorScore:    hit.VectorScore,
			chunkID:        hit.ChunkID,
		})
	}

	// Chunk id breaks ties so the same evidence always numbers the same way, which
	// is what makes a citation reproducible.
	sort.SliceStable(citations, func(i, j int) bool {
		if citations[i].Score != citations[j].Score {
			return citations[i].Score > citations[j].Score
		}
		return citations[i].chunkID < citations[j].chunkID
	})
	for i := range citations {
		citations[i].Index = i + 1
	}
	return citations
}

// RenderContext is the passage block the model reads.
//
// It walks the citations, not the hits, so the numbering cannot drift from the
// citation list: an answer's references would otherwise point at the wrong
// source.
func RenderContext(hits []knowledge.Hit, citations []Citation) string {
	content := make(map[int64]string, len(hits))
	for _, hit := range hits {
		if _, present := content[hit.ChunkID]; !present {
			content[hit.ChunkID] = hit.Content
		}
	}

	var builder strings.Builder
	for _, citation := range citations {
		fmt.Fprintf(&builder, "[%d] %s (part %d)\n%s\n\n",
			citation.Index, citation.DocumentTitle, citation.ChunkOrdinal+1, content[citation.chunkID])
	}
	return strings.TrimSpace(builder.String())
}

// MarkUsed records which citations the answer actually referenced, and reports
// whether any were.
//
// This is the check that separates a grounded answer from one that merely had
// context available: a model can be handed passages and ignore them entirely.
func MarkUsed(answer string, citations []Citation) (marked []Citation, used []int) {
	referenced := make(map[int]bool)
	for _, match := range citationPattern.FindAllStringSubmatch(answer, -1) {
		number, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		referenced[number] = true
	}

	marked = make([]Citation, len(citations))
	copy(marked, citations)
	for i := range marked {
		if referenced[marked[i].Index] {
			marked[i].Used = true
			used = append(used, marked[i].Index)
		}
	}
	sort.Ints(used)
	return marked, used
}

// Conflicted reports whether the strongest passages disagree enough to be worth
// telling the reader about.
//
// It detects *competing sources*, not contradictory statements: two different
// documents that are both genuinely about the question. Deciding whether their
// content actually conflicts needs a model, so this surfaces the situation and
// the synthesis prompt asks for both sides rather than claiming to resolve it.
//
// The comparison is on cosine similarity, not the fused score. Reciprocal rank
// fusion scores compress by construction - the top few results differ by
// fractions of 1/61 - so a relative margin on them fires almost every time and
// says nothing about whether the documents are on the same subject.
func Conflicted(citations []Citation, margin float64) bool {
	if len(citations) < 2 {
		return false
	}
	best, second := citations[0], citations[1]
	if best.DocumentID == second.DocumentID {
		return false
	}
	if best.VectorScore <= 0 {
		// Keyword-only matches carry no semantic signal to compare, so there is
		// no basis for calling them competing answers.
		return false
	}
	return (best.VectorScore-second.VectorScore)/best.VectorScore <= margin
}

// BestDocumentTitle names the strongest source, which is what a follow-up query
// expands from.
func BestDocumentTitle(citations []Citation) string {
	if len(citations) == 0 {
		return ""
	}
	return citations[0].DocumentTitle
}

// BestScore is the strongest fused score, or zero when nothing was retrieved.
func BestScore(citations []Citation) float64 {
	if len(citations) == 0 {
		return 0
	}
	return citations[0].Score
}
