// Distributed MemOS - Core: Cognitive Ranking and Lifecycle
package core

import (
    "regexp"
    "strings"
)

// ExtractEntities is a simple MVP entity extractor: it finds capitalized words
// and common multi-word proper nouns. This is intentionally simple for Phase 3.
func ExtractEntities(text string) []string {
    // remove punctuation except spaces
    re := regexp.MustCompile(`[.,!?;:"()\[\]{}]`)
    cleaned := re.ReplaceAllString(text, "")

    words := strings.Fields(cleaned)
    entitiesMap := make(map[string]struct{})

    for i, w := range words {
        if len(w) == 0 { continue }
        // multiword proper noun: consecutive capitalized words
        if isCapitalized(w) {
            entity := w
            // look ahead
            for j := i+1; j < len(words); j++ {
                if isCapitalized(words[j]) {
                    entity += " " + words[j]
                    i = j
                } else {
                    break
                }
            }
            entitiesMap[entity] = struct{}{}
            continue
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
