package kernel

import (
	"regexp"
	"strings"
)

// thinkingTagPair defines an open/close pair for inline thinking markers.
type thinkingTagPair struct {
	open  string
	close string
}

var thinkingTags = []thinkingTagPair{
	{"<think>", "</think>"},
	{"<|channel>thought", "<channel|>"},
}

// thinkingPatterns matches inline thinking blocks in complete text.
var thinkingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?s)<think>.*?</think>`),
	regexp.MustCompile(`(?s)<\|channel>thought.*?<channel\|>`),
}

// StripThinkingTokens removes all inline thinking markers from complete text.
func StripThinkingTokens(s string) string {
	for _, re := range thinkingPatterns {
		s = re.ReplaceAllString(s, "")
	}
	return strings.TrimSpace(s)
}

// filterThinkingChunk processes a single text chunk in a streaming context.
// closeTag is the expected closing tag when inside a thinking block (empty if not in one).
// Returns the filtered text and the new closeTag state.
func filterThinkingChunk(text string, closeTag string) (string, string) {
	if closeTag != "" {
		// Inside a thinking block — look for the close tag.
		if idx := strings.Index(text, closeTag); idx >= 0 {
			remaining := text[idx+len(closeTag):]
			// Recurse in case another thinking block starts in the same chunk.
			return filterThinkingChunk(remaining, "")
		}
		return "", closeTag
	}

	// Not in a thinking block — look for an opening tag.
	for _, t := range thinkingTags {
		idx := strings.Index(text, t.open)
		if idx < 0 {
			continue
		}
		before := text[:idx]
		after := text[idx+len(t.open):]
		// Check if the close tag appears in the same chunk.
		if endIdx := strings.Index(after, t.close); endIdx >= 0 {
			remaining := after[endIdx+len(t.close):]
			filtered, newClose := filterThinkingChunk(remaining, "")
			return before + filtered, newClose
		}
		// Close tag not yet seen — enter thinking mode.
		return before, t.close
	}

	return text, ""
}
