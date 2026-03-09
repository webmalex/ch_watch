package queue

type OrderedSet struct {
	items []string
	seen  map[string]struct{}
}

func NewOrderedSet() *OrderedSet {
	return &OrderedSet{seen: make(map[string]struct{})}
}

func (s *OrderedSet) Add(value string) bool {
	if _, ok := s.seen[value]; ok {
		return false
	}
	s.seen[value] = struct{}{}
	s.items = append(s.items, value)
	return true
}

func (s *OrderedSet) Drain() []string {
	items := append([]string(nil), s.items...)
	s.items = s.items[:0]
	clear(s.seen)
	return items
}

func (s *OrderedSet) PopFront() (string, bool) {
	if len(s.items) == 0 {
		return "", false
	}
	item := s.items[0]
	copy(s.items, s.items[1:])
	s.items = s.items[:len(s.items)-1]
	delete(s.seen, item)
	return item, true
}

func (s *OrderedSet) Len() int {
	return len(s.items)
}
