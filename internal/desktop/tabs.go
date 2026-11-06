package desktop

import "slices"

// mainTab is one editor tab. Screen is the sidebar id, catalog, or settings.
// A detail tab is logs, a shell, or YAML of one object: it is not a sidebar
// screen, so a navigator click cannot focus it by screen id. One screen has
// one tab. A detail of the same kind and object is focused again, not opened
// twice. Order is the strip order. A new tab is inserted beside the selected
// one using a midpoint, and dragging a tab pins it.
type tabDetail struct {
	kind   string
	key    string
	title  string
	parent string
}

type mainTab struct {
	id        string
	screen    string
	detail    *tabDetail
	ephemeral bool
	order     float64
}

// tabStrip is the main-view tab list. It does not draw and does not touch
// Kubernetes. Split groups and detached windows are not here.
type tabStrip struct {
	tabs     []*mainTab
	selected string
	history  []string
	seq      int
}

func newTabStrip() *tabStrip {
	s := &tabStrip{seq: 1}
	s.tabs = []*mainTab{{id: "t1", screen: "overview", order: 0}}
	s.selected = "t1"
	s.history = []string{"t1"}
	return s
}

func (s *tabStrip) Len() int { return len(s.tabs) }

func (s *tabStrip) Selected() string { return s.selected }

func (s *tabStrip) Screen(id string) string {
	if t := s.tab(id); t != nil {
		return t.screen
	}
	return ""
}

func (s *tabStrip) SelectedScreen() string { return s.Screen(s.selected) }

func (s *tabStrip) Ephemeral(id string) bool {
	t := s.tab(id)
	return t != nil && t.ephemeral
}

func (s *tabStrip) tab(id string) *mainTab {
	for _, t := range s.tabs {
		if t.id == id {
			return t
		}
	}
	return nil
}

func (s *tabStrip) byScreen(screen string) (string, bool) {
	for _, t := range s.tabs {
		if t.detail == nil && t.screen == screen {
			return t.id, true
		}
	}
	return "", false
}

func (s *tabStrip) Detail(id string) *tabDetail {
	t := s.tab(id)
	if t == nil {
		return nil
	}
	return t.detail
}

// Visible returns the tabs left to right.
func (s *tabStrip) Visible() []*mainTab {
	out := append([]*mainTab(nil), s.tabs...)
	slices.SortStableFunc(out, func(a, b *mainTab) int {
		switch {
		case a.order < b.order:
			return -1
		case a.order > b.order:
			return 1
		default:
			return 0
		}
	})
	return out
}

func (s *tabStrip) At(i int) string {
	vis := s.Visible()
	if i < 0 {
		i = len(vis) + i
	}
	if i < 0 || i >= len(vis) {
		return ""
	}
	return vis[i].id
}

func (s *tabStrip) Next() string { return s.step(1) }

func (s *tabStrip) Prev() string { return s.step(-1) }

func (s *tabStrip) step(d int) string {
	vis := s.Visible()
	if len(vis) == 0 {
		return ""
	}
	i := 0
	for n, t := range vis {
		if t.id == s.selected {
			i = n
			break
		}
	}
	i = (i + d) % len(vis)
	if i < 0 {
		i += len(vis)
	}
	return vis[i].id
}

func (s *tabStrip) HasRight(id string) bool {
	t := s.tab(id)
	if t == nil {
		return false
	}
	for _, other := range s.tabs {
		if other.order > t.order {
			return true
		}
	}
	return false
}

// Show focuses the tab of screen, or creates one beside the selected tab.
// An ephemeral create replaces the previous preview tab. A non-ephemeral
// show pins an existing tab. created reports a new tab. closed is the
// preview tab this create replaced, if any.
func (s *tabStrip) Show(screen string, ephemeral bool) (id string, created bool, closed string) {
	if id, ok := s.byScreen(screen); ok {
		if !ephemeral {
			s.pin(id)
		}
		s.Select(id)
		return id, false, ""
	}
	s.seq++
	id = "t" + itoa(s.seq)
	s.tabs = append(s.tabs, &mainTab{
		id:        id,
		screen:    screen,
		ephemeral: ephemeral,
		order:     s.beside(),
	})
	if ephemeral {
		closed = s.ephemeralExcept(id)
	}
	s.Select(id)
	if closed != "" {
		s.drop(closed)
	}
	return id, true, closed
}

// ShowDetail focuses the detail tab of key, or creates a pinned one beside
// the selected tab. An explicit open is not a preview, so it does not replace
// one. created reports a new tab.
func (s *tabStrip) ShowDetail(kind, key, title, parent string) (id string, created bool) {
	for _, t := range s.tabs {
		if t.detail != nil && t.detail.key == key {
			t.detail.title = title
			s.pin(t.id)
			s.Select(t.id)
			return t.id, false
		}
	}
	s.seq++
	id = "t" + itoa(s.seq)
	s.tabs = append(s.tabs, &mainTab{
		id: id,
		detail: &tabDetail{
			kind:   kind,
			key:    key,
			title:  title,
			parent: parent,
		},
		order: s.beside(),
	})
	s.Select(id)
	return id, true
}

// Move places id at visible index to and pins it. Dragging a tab pins it.
// to is the index in the order before the move.
func (s *tabStrip) Move(id string, to int) bool {
	vis := s.Visible()
	from := -1
	for i, t := range vis {
		if t.id == id {
			from = i
			break
		}
	}
	if from < 0 || to < 0 || to >= len(vis) || from == to {
		return false
	}
	item := vis[from]
	rest := make([]*mainTab, 0, len(vis)-1)
	rest = append(rest, vis[:from]...)
	rest = append(rest, vis[from+1:]...)
	if to > len(rest) {
		to = len(rest)
	}
	out := make([]*mainTab, 0, len(vis))
	out = append(out, rest[:to]...)
	out = append(out, item)
	out = append(out, rest[to:]...)
	for i, t := range out {
		t.order = float64(i)
	}
	s.pin(id)
	return true
}

func (s *tabStrip) pin(id string) {
	if t := s.tab(id); t != nil {
		t.ephemeral = false
	}
}

func (s *tabStrip) Pin(id string) { s.pin(id) }

func (s *tabStrip) Select(id string) {
	if s.tab(id) == nil || s.selected == id {
		return
	}
	s.selected = id
	s.history = append(without(s.history, id), id)
}

// Close removes id. The selected tab becomes the last one still in history,
// else the leftmost remaining tab. The returned id is the tab to show.
func (s *tabStrip) Close(id string) string {
	was := s.selected == id
	s.drop(id)
	if !was {
		return s.selected
	}
	if next := s.historyLast(); next != "" {
		s.Select(next)
		return next
	}
	if vis := s.Visible(); len(vis) > 0 {
		s.Select(vis[0].id)
		return vis[0].id
	}
	s.selected = ""
	return ""
}

// CloseOthers removes every tab but id and selects id.
func (s *tabStrip) CloseOthers(id string) []string {
	keep := s.tab(id)
	if keep == nil {
		return nil
	}
	var dead []string
	for _, t := range s.tabs {
		if t.id != id {
			dead = append(dead, t.id)
		}
	}
	s.tabs = []*mainTab{keep}
	s.history = []string{id}
	s.selected = id
	return dead
}

// CloseRight removes tabs to the right of id. If that closed the selected
// tab, the successor is the history, else the leftmost remaining tab.
func (s *tabStrip) CloseRight(id string) []string {
	target := s.tab(id)
	if target == nil {
		return nil
	}
	var dead []string
	keep := make([]*mainTab, 0, len(s.tabs))
	for _, t := range s.tabs {
		if t.order > target.order {
			dead = append(dead, t.id)
		} else {
			keep = append(keep, t)
		}
	}
	s.tabs = keep
	s.history = withoutAll(s.history, dead)
	if s.tab(s.selected) == nil {
		s.selected = ""
		if next := s.historyLast(); next != "" {
			s.Select(next)
		} else if vis := s.Visible(); len(vis) > 0 {
			s.Select(vis[0].id)
		}
	}
	return dead
}

func (s *tabStrip) Clear() []string {
	dead := make([]string, len(s.tabs))
	for i, t := range s.tabs {
		dead[i] = t.id
	}
	s.tabs = nil
	s.history = nil
	s.selected = ""
	return dead
}

func (s *tabStrip) drop(id string) {
	keep := s.tabs[:0]
	for _, t := range s.tabs {
		if t.id != id {
			keep = append(keep, t)
		}
	}
	s.tabs = keep
	s.history = without(s.history, id)
	if s.selected == id {
		s.selected = ""
	}
}

func (s *tabStrip) beside() float64 {
	vis := s.Visible()
	if len(vis) == 0 {
		return 0
	}
	for i, t := range vis {
		if t.id != s.selected {
			continue
		}
		if i+1 < len(vis) {
			return (t.order + vis[i+1].order) / 2
		}
		return t.order + 1
	}
	return vis[len(vis)-1].order + 1
}

func (s *tabStrip) ephemeralExcept(id string) string {
	for _, t := range s.tabs {
		if t.ephemeral && t.id != id {
			return t.id
		}
	}
	return ""
}

func (s *tabStrip) historyLast() string {
	for i := len(s.history) - 1; i >= 0; i-- {
		if s.tab(s.history[i]) != nil {
			return s.history[i]
		}
	}
	return ""
}

func without(ids []string, id string) []string {
	out := ids[:0]
	for _, item := range ids {
		if item != id {
			out = append(out, item)
		}
	}
	return out
}

func withoutAll(ids, dead []string) []string {
	out := ids[:0]
	for _, id := range ids {
		if !slices.Contains(dead, id) {
			out = append(out, id)
		}
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
