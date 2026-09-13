// Package deltamini is a Go port of Tidal's deltaMini (tidal-core,
// Sound.Tidal.Pattern). deltaMini rewrites pattern source code by wrapping
// every string literal in a (deltaContext column line "...") call, which
// shifts the source positions of the mininotation events inside the literal
// to buffer coordinates. It relies on OverloadedStrings (set by the boot
// script) so GHCi dispatches deltaContext to the Pattern instance.
//
// The port mirrors the Haskell implementation exactly, including its
// treatment of escaped quotes (any '"' terminates a literal) and
// unterminated literals (no closing text is appended).
package deltamini

import (
	"strconv"
	"strings"
)

// Delta rewrites code so that mininotation events report positions relative
// to the buffer. startLine is the 0-based buffer line the code starts on;
// the start column is always 0 because evaluated blocks always start at the
// beginning of a buffer line.
func Delta(code string, startLine int) string {
	var b strings.Builder
	outside(&b, code, 0, startLine)
	return b.String()
}

// outside scans code outside string literals. When it hits a quote it emits
// the deltaContext wrapper and switches to inside. column and line are the
// 0-based coordinates of the next rune to consume.
func outside(b *strings.Builder, code string, column, line int) {
	for i, r := range code {
		switch r {
		case '"':
			b.WriteString("(deltaContext ")
			b.WriteString(strconv.Itoa(column))
			b.WriteByte(' ')
			b.WriteString(strconv.Itoa(line))
			b.WriteString(" \"")
			inside(b, code[i+1:], column+1, line)
			return
		case '\n':
			b.WriteRune('\n')
			column = 0
			line++
		default:
			b.WriteRune(r)
			column++
		}
	}
}

// inside scans code within a string literal. When it hits the closing quote
// it closes the deltaContext call and returns to outside.
func inside(b *strings.Builder, code string, column, line int) {
	for i, r := range code {
		switch r {
		case '"':
			b.WriteString("\")")
			outside(b, code[i+1:], column+1, line)
			return
		case '\n':
			b.WriteRune('\n')
			column = 0
			line++
		default:
			b.WriteRune(r)
			column++
		}
	}
}
