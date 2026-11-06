// Package deltamini is a Go port of Tidal's deltaMini (tidal-core,
// Sound.Tidal.Pattern). deltaMini rewrites pattern source code by wrapping
// every string literal in a (deltaContext column line "...") call, which
// shifts the source positions of the mininotation events inside the literal
// to buffer coordinates. It relies on OverloadedStrings (set by the boot
// script) so GHCi dispatches deltaContext to the Pattern instance.
//
// The port scans like the Haskell lexer: quotes inside line comments and
// nested block comments are comment text, and a quote escaped by an odd
// backslash run stays inside its literal. Unterminated literals mirror
// deltaMini (no closing text is appended).
package deltamini

import (
	"strconv"
	"strings"
)

// symbolChars are Haskell's operator symbol characters: "--" followed by
// one of them lexes as an operator (e.g. -->), not a comment.
const symbolChars = "!#$%&*+./<=>?@\\^|-~"

func isSymbol(r rune) bool {
	return strings.ContainsRune(symbolChars, r)
}

// Delta rewrites code so that mininotation events report positions relative
// to the buffer. startLine is the 0-based buffer line the code starts on;
// the start column is always 0 because evaluated blocks always start at the
// beginning of a buffer line.
func Delta(code string, startLine int) string {
	var b strings.Builder
	s := scanner{b: &b, line: startLine}
	s.run(code)
	return b.String()
}

// scanner walks code once, emitting it verbatim except for the
// deltaContext wrappers it opens and closes around string literals.
// column and line are the 0-based coordinates of the next rune to emit.
type scanner struct {
	b      *strings.Builder
	column int
	line   int
}

// emit writes one rune and advances the buffer position.
func (s *scanner) emit(r rune) {
	s.b.WriteRune(r)
	if r == '\n' {
		s.column = 0
		s.line++
		return
	}
	s.column++
}

func (s *scanner) run(code string) {
	rs := []rune(code)
	for i := 0; i < len(rs); {
		switch {
		case lineCommentStart(rs, i):
			i = s.emitLineComment(rs, i)
		case rs[i] == '{' && i+1 < len(rs) && rs[i+1] == '-':
			i = s.emitBlockComment(rs, i)
		case rs[i] == '"':
			i = s.emitLiteral(rs, i)
		default:
			s.emit(rs[i])
			i++
		}
	}
}

// lineCommentStart reports whether rs[i:] starts a Haskell line comment:
// two dashes not followed by a symbol character, which would make them
// part of an operator.
func lineCommentStart(rs []rune, i int) bool {
	if rs[i] != '-' || i+1 >= len(rs) || rs[i+1] != '-' {
		return false
	}
	if i+2 >= len(rs) {
		return true
	}
	return !isSymbol(rs[i+2])
}

// emitLineComment emits the comment verbatim, quotes and all, and
// returns the index of the rune after it (the newline, if any).
func (s *scanner) emitLineComment(rs []rune, i int) int {
	for ; i < len(rs) && rs[i] != '\n'; i++ {
		s.emit(rs[i])
	}
	return i
}

// emitBlockComment emits a nested {- -} comment verbatim, quotes and
// all, and returns the index after its closing braces. An unclosed
// block comment consumes the rest of the input.
func (s *scanner) emitBlockComment(rs []rune, i int) int {
	depth := 0
	for ; i < len(rs); i++ {
		switch {
		case rs[i] == '{' && i+1 < len(rs) && rs[i+1] == '-':
			depth++
			s.emit(rs[i])
			i++
			s.emit(rs[i])
		case rs[i] == '-' && i+1 < len(rs) && rs[i+1] == '}':
			depth--
			s.emit(rs[i])
			i++
			s.emit(rs[i])
			if depth == 0 {
				return i + 1
			}
		default:
			s.emit(rs[i])
		}
	}
	return i
}

// emitLiteral rewrites the string literal starting at rs[i] (an opening
// quote) as a deltaContext call and returns the index after the closing
// quote. A quote escaped by an odd backslash run stays in the literal;
// an unterminated literal consumes the rest of the input like deltaMini.
func (s *scanner) emitLiteral(rs []rune, i int) int {
	s.b.WriteString("(deltaContext ")
	s.b.WriteString(strconv.Itoa(s.column))
	s.b.WriteByte(' ')
	s.b.WriteString(strconv.Itoa(s.line))
	s.b.WriteString(" \"")
	s.column++ // the opening quote
	slashes := 0
	i++
	for ; i < len(rs); i++ {
		r := rs[i]
		if r == '\\' {
			slashes++
			s.emit(r)
			continue
		}
		if r == '"' && slashes%2 == 0 {
			s.b.WriteString("\")")
			s.column++ // the closing quote
			return i + 1
		}
		slashes = 0
		s.emit(r)
	}
	return i
}
