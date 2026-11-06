package textarea

// maxWrapEntries bounds the memoizer: entries are keyed by line index for a
// single buffer revision and width, so they are naturally limited by the
// line count. The ceiling only guards a model whose line limit is disabled
// from growing the cache without bound.
const maxWrapEntries = 2048

// wrapCache memoizes soft-wrapped rows by buffer line index. Entries are
// valid only for the revision and width they were wrapped at: any edit or
// resize invalidates them all, so repeated frames (cursor blinks, highlight
// refreshes) cost one map lookup per line instead of re-wrapping, and an
// edit only pays for the first frame that renders it.
type wrapCache struct {
	rev   uint64
	width int
	rows  map[int][][]rune
}

func newWrapCache() *wrapCache {
	return &wrapCache{}
}

// get returns the memoized rows for a line, if they were wrapped for the
// given revision and width.
func (c *wrapCache) get(index int, rev uint64, width int) ([][]rune, bool) {
	if c.rev != rev || c.width != width || c.rows == nil {
		return nil, false
	}
	rows, ok := c.rows[index]
	return rows, ok
}

// put stores a line's rows for the given revision and width, replacing every
// older entry when either changed.
func (c *wrapCache) put(index int, rev uint64, width int, rows [][]rune) {
	if c.rev != rev || c.width != width || c.rows == nil {
		c.rev, c.width, c.rows = rev, width, make(map[int][][]rune)
	}
	if len(c.rows) > maxWrapEntries {
		clear(c.rows)
	}
	c.rows[index] = rows
}

// clear drops every entry so nothing is served until rows are memoized
// again.
func (c *wrapCache) clear() {
	c.rows = nil
}
