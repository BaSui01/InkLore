package domain

import (
	"strings"
	"unicode/utf8"
)

const MaxSnippetRunes = 500

func MatchSnippet(title, body, query string) (snippet string, startOffset int, endOffset int) {
	qLower := strings.ToLower(query)
	bodyLower := strings.ToLower(body)
	qRunes := utf8.RuneCountInString(query)

	byteIdx := strings.Index(bodyLower, qLower)
	if byteIdx != -1 {
		bodyRunes := []rune(body)
		startOffset = utf8.RuneCountInString(body[:byteIdx])
		endOffset = startOffset + qRunes

		totalRunes := len(bodyRunes)
		if totalRunes <= MaxSnippetRunes {
			snippet = body
			return
		}

		preContext := 150
		if qRunes > 200 {
			preContext = 50
		}
		winStart := startOffset - preContext
		if winStart < 0 {
			winStart = 0
		}
		winEnd := winStart + MaxSnippetRunes
		if winEnd > totalRunes {
			winEnd = totalRunes
			winStart = winEnd - MaxSnippetRunes
			if winStart < 0 {
				winStart = 0
			}
		}
		snippet = string(bodyRunes[winStart:winEnd])
		return
	}

	// Only matched title, or fallback
	snippet = title
	startOffset = 0
	endOffset = 0
	return
}
