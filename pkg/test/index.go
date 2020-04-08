package test

import "strings"

// Index is a in-memory mock implementation for testing.
// Better than using a mocking library? ¯\_(ツ)_/¯.
type Index struct {
	Data             []string
	ForceSearchError error
}

// Search is a test implementation of an index searching for matches by prefix.
func (s *Index) Search(search string) ([]string, error) {
	if s.ForceSearchError != nil {
		return nil, s.ForceSearchError
	}
	var matches []string
	for _, item := range s.Data {
		if strings.HasPrefix(item, search) {
			matches = append(matches, item)
		}
	}
	return matches, nil
}
