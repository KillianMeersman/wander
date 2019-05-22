package util

type StringSet map[string]struct{}

func (s StringSet) Add(value string) {
	s[value] = struct{}{}
}

func (s StringSet) Contains(value string) bool {
	_, ok := s[value]
	return ok
}
