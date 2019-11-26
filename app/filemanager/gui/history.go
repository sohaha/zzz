package gui

// History history info
type History struct {
	RowIdx int
	Path   string
}

// HistoryManager have the move history
// TODO limit max history
type HistoryManager struct {
	idx       int
	histories []*History
}

// NewHistoryManager new history manager
func NewHistoryManager() *HistoryManager {
	return &HistoryManager{}
}

// Save save the move history
func (h *HistoryManager) Save(rowIdx int, path string) {
	count := len(h.histories)

	history := &History{RowIdx: rowIdx, Path: path}
	// if not have history
	if count == 0 {
		h.histories = append(h.histories, history)
	} else {
		h.histories = append(h.histories, history)
		h.idx++
	}
}

// Previous return the previous history
func (h *HistoryManager) Previous() *History {
	count := len(h.histories)
	if count == 0 {
		return nil
	}

	h.idx--
	if h.idx < 0 {
		h.idx = 0
	}
	return h.histories[h.idx]
}

// Next return the next history
func (h *HistoryManager) Next() *History {
	count := len(h.histories)
	if count == 0 {
		return nil
	}

	h.idx++
	if h.idx >= count {
		h.idx = count - 1
	}
	return h.histories[h.idx]
}
