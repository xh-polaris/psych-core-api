package filter

import (
	"sync"
)

type DFANode struct {
	Children map[rune]*DFANode
	IsEnd    bool
}

type DFAFilter struct {
	root *DFANode
	mu   sync.RWMutex
}

func NewDFAFilter() *DFAFilter {
	return &DFAFilter{
		root: &DFANode{
			Children: make(map[rune]*DFANode),
		},
	}
}

// AddWord 向DFA树添加词汇
func (f *DFAFilter) AddWord(word string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	node := f.root
	for _, char := range word {
		if node.Children == nil {
			node.Children = make(map[rune]*DFANode)
		}
		if next, ok := node.Children[char]; ok {
			node = next
		} else {
			newNode := &DFANode{
				Children: make(map[rune]*DFANode),
			}
			node.Children[char] = newNode
			node = newNode
		}
	}
	node.IsEnd = true
}

// AddWords 批量添加词汇
func (f *DFAFilter) AddWords(words []string) {
	for _, word := range words {
		f.AddWord(word)
	}
}

// Match 检查文本中是否包含任一过滤词，返回第一个匹配到的词
func (f *DFAFilter) Match(text string) (bool, string) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	runes := []rune(text)
	length := len(runes)

	for i := 0; i < length; i++ {
		node := f.root
		for j := i; j < length; j++ {
			if next, ok := node.Children[runes[j]]; ok {
				node = next
				if node.IsEnd {
					return true, string(runes[i : j+1])
				}
			} else {
				break
			}
		}
	}
	return false, ""
}

// Replace 替换文本中的所有过滤词
func (f *DFAFilter) Replace(text string, replacement rune) string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	runes := []rune(text)
	length := len(runes)
	result := make([]rune, length)
	copy(result, runes)

	for i := 0; i < length; i++ {
		node := f.root
		matchLen := 0
		for j := i; j < length; j++ {
			if next, ok := node.Children[runes[j]]; ok {
				node = next
				if node.IsEnd {
					matchLen = j - i + 1
				}
			} else {
				break
			}
		}

		if matchLen > 0 {
			for k := 0; k < matchLen; k++ {
				result[i+k] = replacement
			}
			i += matchLen - 1
		}
	}
	return string(result)
}

// IsMatched 精确匹配整个词
func (f *DFAFilter) IsMatched(word string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	node := f.root
	for _, char := range word {
		if next, ok := node.Children[char]; ok {
			node = next
		} else {
			return false
		}
	}
	return node.IsEnd
}
