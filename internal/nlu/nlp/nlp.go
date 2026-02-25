package nlp

import (
	"strings"
)

type Tag string

const (
	TagNoun  Tag = "NN"
	TagVerb  Tag = "VB"
	TagAdj   Tag = "JJ"
	TagAdv   Tag = "RB"
	TagDet   Tag = "DT"
	TagPrep  Tag = "IN"
	TagPron  Tag = "PRP"
	TagNum   Tag = "CD"
	TagPunct Tag = "."
	TagUnk   Tag = "UNK"
)

type Token struct {
	Text string
	Tag  Tag
}

// SimplePOSTagger implements a rule-based/lookup-based tagger.
// It is lightweight and dependency-free, suitable for the standard library constraint.
type SimplePOSTagger struct {
	lexicon map[string]Tag
}

func NewPOSTagger() *SimplePOSTagger {
	return &SimplePOSTagger{
		lexicon: map[string]Tag{
			"weather": TagNoun, "temperature": TagNoun, "forecast": TagNoun,
			"alarm": TagNoun, "timer": TagNoun, "clock": TagNoun,
			"device": TagNoun, "lights": TagNoun, "fan": TagNoun,
			"paris": TagNoun, "london": TagNoun, "tokyo": TagNoun,

			"set": TagVerb, "create": TagVerb, "make": TagVerb,
			"turn": TagVerb, "switch": TagVerb, "get": TagVerb, "tell": TagVerb,
			"show": TagVerb,

			"in": TagPrep, "at": TagPrep, "for": TagPrep, "on": TagPrep, "off": TagPrep, "to": TagPrep,

			"the": TagDet, "a": TagDet, "an": TagDet,

			"quick": TagAdj, "slow": TagAdj, "good": TagAdj, "bad": TagAdj,

			"i": TagPron, "you": TagPron, "me": TagPron,

			"now": TagAdv, "later": TagAdv,
		},
	}
}

func (t *SimplePOSTagger) Tag(input string) []Token {
	// Simple whitespace tokenizer
	// In a real scenario, use a better tokenizer that handles punctuation
	fields := strings.Fields(strings.ToLower(input))
	tokens := make([]Token, 0, len(fields))

	for _, text := range fields {
		text = strings.Trim(text, ",.?!") // Basic cleanup
		if text == "" {
			continue
		}

		tag := TagUnk
		if val, ok := t.lexicon[text]; ok {
			tag = val
		} else {
			// Basic morphology rules
			if strings.HasSuffix(text, "ing") || strings.HasSuffix(text, "ed") {
				tag = TagVerb
			} else if strings.HasSuffix(text, "ly") {
				tag = TagAdv
			} else if isNumber(text) {
				tag = TagNum
			} else {
				// Fallback to noun for unknown words (common baseline)
				tag = TagNoun
			}
		}
		tokens = append(tokens, Token{Text: text, Tag: tag})
	}
	return tokens
}

func isNumber(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
