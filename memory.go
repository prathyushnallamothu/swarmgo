package swarmgo

import (
	"encoding/json"
	"sync"
	"time"
)

// Memory represents a single memory entry
type Memory struct {
	Content    string                 `json:"content"`    // The actual memory content
	Type       string                 `json:"type"`       // Type of memory (e.g., "conversation", "fact", "task")
	Context    map[string]interface{} `json:"context"`    // Associated context
	Timestamp  time.Time              `json:"timestamp"`  // When the memory was created
	Importance float64                `json:"importance"` // Importance score (0-1)
	References []string               `json:"references"` // References to related memories
}

func (m *Memory) Equals(other *Memory) bool {
	return m.Content == other.Content &&
		m.Type == other.Type &&
		m.Importance == other.Importance
}

// MemoryStore manages agent memories
type MemoryStore struct {
	shortTerm []Memory            // Recent memories (FIFO buffer)
	longTerm  map[string][]Memory // Organized long-term memories
	maxShort  int                 // Maximum number of short-term memories
	mu        sync.RWMutex        // For thread safety
}

// NewMemoryStore creates a new memory store with default settings
func NewMemoryStore(maxShortTerm int) *MemoryStore {
	return &MemoryStore{
		shortTerm: make([]Memory, 0),
		longTerm:  make(map[string][]Memory),
		maxShort:  maxShortTerm,
	}
}

// AddMemory adds a new memory to both short and long-term storage
func (ms *MemoryStore) AddMemory(memory Memory) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Add to short-term memory
	ms.shortTerm = append(ms.shortTerm, memory)
	if len(ms.shortTerm) > ms.maxShort {
		// Remove oldest memory when capacity is exceeded
		ms.shortTerm = ms.shortTerm[1:]
	}

	// Add to long-term memory
	if memory.Type != "" {
		ms.longTerm[memory.Type] = append(ms.longTerm[memory.Type], memory)
	}
}

func (ms *MemoryStore) RemoveMemory(content string, _type string) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	// Remove from short-term memory
	for i, m := range ms.shortTerm {
		if m.Content == content && m.Type == _type {
			ms.shortTerm = append(ms.shortTerm[:i], ms.shortTerm[i+1:]...)
			break
		}
	}

	// Remove from long-term memory
	if _type != "" {
		for i, m := range ms.longTerm[_type] {
			if m.Content == content && m.Type == _type {
				ms.longTerm[_type] = append(ms.longTerm[_type][:i], ms.longTerm[_type][i+1:]...)
				break
			}
		}
	}
}

// GetRecentMemories retrieves the n most recent memories
func (ms *MemoryStore) GetRecentMemories(n int) []Memory {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if n > len(ms.shortTerm) {
		n = len(ms.shortTerm)
	}

	start := len(ms.shortTerm) - n
	if start < 0 {
		start = 0
	}

	return ms.shortTerm[start:]
}

// SearchMemories searches for memories based on type and context
func (ms *MemoryStore) SearchMemories(memoryType string, context map[string]interface{}) []Memory {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if memories, exists := ms.longTerm[memoryType]; exists {
		if context == nil {
			return memories
		}

		// Filter memories by context match
		var filtered []Memory
		for _, memory := range memories {
			if matchContext(memory.Context, context) {
				filtered = append(filtered, memory)
			}
		}
		return filtered
	}
	return nil
}

// matchContext checks if a memory's context matches the search context
func matchContext(memContext, searchContext map[string]interface{}) bool {
	for key, searchVal := range searchContext {
		if memVal, exists := memContext[key]; !exists || memVal != searchVal {
			return false
		}
	}
	return true
}

// SerializeMemories serializes all memories to JSON
func (ms *MemoryStore) SerializeMemories() ([]byte, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	data := struct {
		ShortTerm []Memory            `json:"short_term"`
		LongTerm  map[string][]Memory `json:"long_term"`
	}{
		ShortTerm: ms.shortTerm,
		LongTerm:  ms.longTerm,
	}

	return json.Marshal(data)
}

// LoadMemories loads memories from JSON data
func (ms *MemoryStore) LoadMemories(data []byte) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	var loaded struct {
		ShortTerm []Memory            `json:"short_term"`
		LongTerm  map[string][]Memory `json:"long_term"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	ms.shortTerm = loaded.ShortTerm
	ms.longTerm = loaded.LongTerm
	return nil
}
