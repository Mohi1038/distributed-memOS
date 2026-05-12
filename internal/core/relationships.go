// Distributed MemOS - Core: Cognitive Ranking and Lifecycle
package core

import (
    "regexp"
    "strings"
)

var domainKeywords = []string{
	"Redis", "NATS", "Postgres", "SQL", "Rust", "Go", "Golang", "Python", 
	"Kubernetes", "Docker", "Replication", "MemOS", "RAG", "LLM", "Vector",
	"Anti-Entropy", "Consensus", "Distributed", "Shard", "Cluster",
}

// ExtractEntities finds domain-specific keywords and proper nouns.
func ExtractEntities(text string) []string {
	entitiesMap := make(map[string]struct{})
	lowerText := strings.ToLower(text)

	// 1. Domain keyword spotting
	for _, kw := range domainKeywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			entitiesMap[kw] = struct{}{}
		}
	}

	// 2. Proper Noun detection (Capitalized words)
	re := regexp.MustCompile(`[.,!?;:"()\[\]{}]`)
	cleaned := re.ReplaceAllString(text, "")
	words := strings.Fields(cleaned)

	for i := 0; i < len(words); i++ {
		w := words[i]
		if isCapitalized(w) {
			entity := w
			for j := i + 1; j < len(words); j++ {
				if isCapitalized(words[j]) {
					entity += " " + words[j]
					i = j
				} else {
					break
				}
			}
			// Only add if it's not a common start-of-sentence word
			if len(entity) > 2 {
				entitiesMap[entity] = struct{}{}
			}
		}
	}

	var entities []string
	for e := range entitiesMap {
		entities = append(entities, e)
	}
	return entities
}

func isCapitalized(s string) bool {
    if len(s) == 0 { return false }
    r := []rune(s)
    return r[0] >= 'A' && r[0] <= 'Z'
}
