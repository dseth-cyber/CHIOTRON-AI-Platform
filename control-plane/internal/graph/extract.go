package graph

import (
	"sort"
	"strings"
	"unicode"
)

// Extraction limits. Without them a long document produces a hairball: every
// entity joined to every other, which is a graph with no information in it.
const (
	maxEntitiesPerChunk = 12
	maxPhraseWords      = 4
	minEntityLength     = 3
	minAcronymLength    = 2
	maxAcronymLength    = 6
)

// sentenceStarters are words that begin a sentence in title case and would
// otherwise be extracted as entities on that basis alone.
var sentenceStarters = map[string]bool{
	"the": true, "a": true, "an": true, "this": true, "that": true, "these": true,
	"those": true, "it": true, "they": true, "we": true, "you": true, "he": true,
	"she": true, "there": true, "here": true, "if": true, "when": true, "while": true,
	"and": true, "or": true, "but": true, "for": true, "to": true, "in": true,
	"on": true, "at": true, "by": true, "with": true, "from": true, "as": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "do": true,
	"does": true, "did": true, "not": true, "no": true, "every": true, "each": true,
	"all": true, "any": true, "use": true, "using": true, "only": true, "must": true,
	"may": true, "can": true, "should": true, "will": true, "would": true,
}

// Source is the document a projection is being built from. The ACL triple is
// copied onto every node and edge, so a traversal can filter without joining
// back to the document.
type Source struct {
	DocumentID     string
	Title          string
	Classification string
	CompanyID      string
	Department     string
}

// Extract builds a projection from a document's chunks.
//
// Extraction is deterministic rather than model-driven. A rule can be tested and
// explained, and the same document always produces the same graph - which is what
// makes a traversal reproducible and an edge weight meaningful. Typed relations
// need a capable model; until one is configured, edges record co-occurrence and
// say so rather than inventing a label.
func Extract(source Source, chunks []string) Projection {
	document := Node{
		Kind:           KindDocument,
		Name:           source.Title,
		Normalised:     Normalise(source.Title),
		Classification: source.Classification,
		CompanyID:      source.CompanyID,
		Department:     source.Department,
		Properties:     map[string]any{"documentId": source.DocumentID},
	}

	// Counters keyed by normalised name, so casing variants collapse into one node.
	type candidate struct {
		display string
		kind    string
		count   int
	}
	entities := map[string]*candidate{}
	mentions := map[string]map[int]int{}
	pairs := map[string]int{}

	for ordinal, chunk := range chunks {
		found := entitiesIn(chunk)
		for name, kind := range found {
			key := Normalise(name)
			if existing, ok := entities[key]; ok {
				existing.count++
			} else {
				entities[key] = &candidate{display: name, kind: kind, count: 1}
			}
			if mentions[key] == nil {
				mentions[key] = map[int]int{}
			}
			mentions[key][ordinal]++
		}

		// Co-occurrence is within a chunk, not a document: two entities in the same
		// passage are plausibly related, two in a 50-page document are not.
		names := make([]string, 0, len(found))
		for name := range found {
			names = append(names, Normalise(name))
		}
		sort.Strings(names)
		for i := 0; i < len(names); i++ {
			for j := i + 1; j < len(names); j++ {
				pairs[names[i]+"\x00"+names[j]]++
			}
		}
	}

	projection := Projection{Nodes: []Node{document}}

	// Deterministic order: the projection is written in one transaction and a
	// stable order makes the write reproducible and easier to read in a diff.
	keys := make([]string, 0, len(entities))
	for key := range entities {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		found := entities[key]
		projection.Nodes = append(projection.Nodes, Node{
			Kind:           found.kind,
			Name:           found.display,
			Normalised:     key,
			Classification: source.Classification,
			CompanyID:      source.CompanyID,
			Department:     source.Department,
			Properties:     map[string]any{"observed": found.count},
		})
		projection.Edges = append(projection.Edges, Edge{
			SourceID:       key,
			TargetID:       document.Normalised,
			Relation:       RelationMentionedIn,
			Weight:         found.count,
			Classification: source.Classification,
			CompanyID:      source.CompanyID,
			Department:     source.Department,
		})
		for ordinal, occurrences := range mentions[key] {
			projection.Mentions = append(projection.Mentions, Mention{
				NodeName: key, DocumentID: source.DocumentID,
				ChunkOrdinal: ordinal, Occurrences: occurrences,
			})
		}
	}

	pairKeys := make([]string, 0, len(pairs))
	for key := range pairs {
		pairKeys = append(pairKeys, key)
	}
	sort.Strings(pairKeys)
	for _, key := range pairKeys {
		left, right, _ := strings.Cut(key, "\x00")
		projection.Edges = append(projection.Edges, Edge{
			SourceID:       left,
			TargetID:       right,
			Relation:       RelationCoOccursWith,
			Weight:         pairs[key],
			Classification: source.Classification,
			CompanyID:      source.CompanyID,
			Department:     source.Department,
		})
	}

	sort.Slice(projection.Mentions, func(i, j int) bool {
		if projection.Mentions[i].NodeName != projection.Mentions[j].NodeName {
			return projection.Mentions[i].NodeName < projection.Mentions[j].NodeName
		}
		return projection.Mentions[i].ChunkOrdinal < projection.Mentions[j].ChunkOrdinal
	})
	return projection
}

// entitiesIn finds candidate entities in one chunk, mapping display name to kind.
func entitiesIn(chunk string) map[string]string {
	found := map[string]string{}

	for _, sentence := range splitSentences(chunk) {
		words := strings.Fields(sentence)
		for index := 0; index < len(words); index++ {
			word := trimWord(words[index])
			if word == "" {
				continue
			}

			if isAcronym(word) {
				found[word] = KindAcronym
				continue
			}
			// The first word of a sentence is capitalised by grammar, so on its own
			// it is no evidence of a name.
			if index == 0 && !isAcronym(word) {
				continue
			}
			if !isTitleCase(word) || sentenceStarters[strings.ToLower(word)] {
				continue
			}

			// Greedily extend through following title-case words, so "Control Plane"
			// is one entity rather than two.
			//
			// An acronym is allowed inside a phrase because names like "Enterprise
			// AI Platform" are everywhere in this corpus, but never at the end:
			// extending into a trailing acronym would fuse "Control Plane" with the
			// "VM4" it merely runs on.
			phrase := []string{word}
			lastKept := index
			for next := index + 1; next < len(words) && len(phrase) < maxPhraseWords; next++ {
				candidate := trimWord(words[next])
				if candidate == "" || sentenceStarters[strings.ToLower(candidate)] {
					break
				}
				switch {
				case isTitleCase(candidate):
					phrase = append(phrase, candidate)
					lastKept = next
				case isAcronym(candidate):
					phrase = append(phrase, candidate)
				default:
					next = len(words)
				}
			}
			// Trim back to the last title-case token, releasing any trailing
			// acronym to be recognised on its own.
			for len(phrase) > 1 && isAcronym(phrase[len(phrase)-1]) {
				phrase = phrase[:len(phrase)-1]
			}
			index = lastKept

			name := strings.Join(phrase, " ")
			if len(name) >= minEntityLength {
				found[name] = KindEntity
			}
		}
		if len(found) >= maxEntitiesPerChunk {
			break
		}
	}
	return found
}

func splitSentences(chunk string) []string {
	return strings.FieldsFunc(chunk, func(char rune) bool {
		return char == '.' || char == '!' || char == '?' || char == '\n' || char == ';' || char == ':'
	})
}

// trimWord strips punctuation and possessives so "VM4," and "VM4" are one term.
func trimWord(word string) string {
	trimmed := strings.Trim(word, `.,;:!?()[]{}"'*_`)
	trimmed = strings.TrimSuffix(trimmed, "'s")
	return trimmed
}

func isTitleCase(word string) bool {
	runes := []rune(word)
	if len(runes) < 2 || !unicode.IsUpper(runes[0]) {
		return false
	}
	// Reject SHOUTING, which is handled as an acronym instead.
	for _, char := range runes[1:] {
		if unicode.IsLower(char) {
			return true
		}
	}
	return false
}

// isAcronym accepts short all-caps tokens, including ones carrying digits such as
// VM4 or S3, which are exactly the identifiers this corpus is full of.
func isAcronym(word string) bool {
	runes := []rune(word)
	if len(runes) < minAcronymLength || len(runes) > maxAcronymLength {
		return false
	}
	letters := 0
	for _, char := range runes {
		switch {
		case unicode.IsUpper(char):
			letters++
		case unicode.IsDigit(char):
		default:
			return false
		}
	}
	return letters > 0
}
