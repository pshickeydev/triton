package textarea

// Meaningful values for the line-content hash: FNV-1a 64-bit parameters.
const (
	fnvOffsetBasis uint64 = 1469598103934665603
	fnvPrime       uint64 = 1099511628211

	// maxWrapEntries bounds the memoizer: identical lines share an entry, so
	// normal buffers stay far below it, and a long session cannot grow it
	// without limit.
	maxWrapEntries = 2048
)

// wrapKey identifies one memoized soft-wrap result: a line's contents (by
// hash and length) plus the width it was wrapped at. A resize invalidates
// every entry; an edit invalidates only the edited line.
type wrapKey struct {
	hash  uint64
	size  int
	width int
}

// wrapCache memoizes soft-wrapped rows per line contents.
type wrapCache struct {
	entries map[wrapKey][][]rune
}

func newWrapCache() *wrapCache {
	return &wrapCache{entries: make(map[wrapKey][][]rune)}
}

// hashLine folds a line's runes into a 64-bit FNV-1a value.
func hashLine(runes []rune) uint64 {
	h := fnvOffsetBasis
	for _, r := range runes {
		h = (h ^ uint64(r)) * fnvPrime
	}
	return h
}

func wrapKeyOf(runes []rune, width int) wrapKey {
	return wrapKey{hash: hashLine(runes), size: len(runes), width: width}
}

// get returns the cached rows for a line, if any.
func (c *wrapCache) get(runes []rune, width int) ([][]rune, bool) {
	rows, ok := c.entries[wrapKeyOf(runes, width)]
	return rows, ok
}

// put stores a line's rows, dropping everything when the memoizer outgrows
// its ceiling rather than letting a long session grow it without bound.
func (c *wrapCache) put(runes []rune, width int, rows [][]rune) {
	if len(c.entries) > maxWrapEntries {
		c.clear()
	}
	c.entries[wrapKeyOf(runes, width)] = rows
}

// clear empties the memoizer.
func (c *wrapCache) clear() {
	clear(c.entries)
}
