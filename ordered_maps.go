package brigodier

import (
	"encoding/json"
	"github.com/emirpasic/gods/maps/linkedhashmap"
)

type (
	// Container is base interface that container structures implement.
	Container interface {
		// Empty returns true if map does not contain any elements
		Empty() bool
		// Size returns number of elements in the map.
		Size() int
		// Clear removes all elements from the map.
		Clear()
	}
	// CommandNodeStringMap holds the elements in a regular hash table,
	// and uses doubly-linked list to store key ordering.
	CommandNodeStringMap interface {
		// Put inserts key-value pair into the map.
		Put(key CommandNode, value string)
		// Get searches the element in the map by key and returns its value or nil if key is not found in tree.
		// Second return parameter is true if key was found, otherwise false.
		Get(key CommandNode) (value string, found bool)
		// Remove removes the element from the map by key.
		Remove(key CommandNode)
		// Keys returns all keys in-order
		Keys() []CommandNode
		// Values returns all values in-order based on the key.
		Values() []string

		// Range calls the given function once for each element
		// until f returns false, passing that element's key and value.
		Range(f func(key CommandNode, value string) bool)

		Container
	}

	// StringCommandNodeMap holds the elements in a regular hash table,
	// and uses doubly-linked list to store key ordering.
	StringCommandNodeMap interface {
		// Put inserts key-value pair into the map.
		Put(key string, value CommandNode)
		// Get searches the element in the map by key and returns its value or nil if key is not found in tree.
		// Second return parameter is true if key was found, otherwise false.
		Get(key string) (value CommandNode, found bool)
		// Remove removes the element from the map by key.
		Remove(key string)
		// Keys returns all keys in-order
		Keys() []string
		// Values returns all values in-order based on the key.
		Values() []CommandNode

		// Range calls the given function once for each element
		// until f returns false, passing that element's key and value.
		Range(f func(key string, value CommandNode) bool)

		Container
	}
)

// NewCommandNodeStringMap returns a new CommandNodeStringMap.
func NewCommandNodeStringMap() CommandNodeStringMap {
	return &commandNodeStringMap{linkedhashmap.New()}
}

// NewStringCommandNodeMap returns a new StringCommandNodeMap.
func NewStringCommandNodeMap() StringCommandNodeMap {
	return &stringCommandNodeMap{linkedhashmap.New()}
}

type commandNodeStringMap struct{ *linkedhashmap.Map }

func (m *commandNodeStringMap) MarshalJSON() ([]byte, error)    { return m.Map.ToJSON() }
func (m *commandNodeStringMap) UnmarshalJSON(data []byte) error { return m.Map.FromJSON(data) }

var _ CommandNodeStringMap = (*commandNodeStringMap)(nil)
var _ json.Marshaler = (*commandNodeStringMap)(nil)
var _ json.Unmarshaler = (*commandNodeStringMap)(nil)

func (m *commandNodeStringMap) Range(f func(key CommandNode, value string) bool) {
	m.Map.All(func(key interface{}, value interface{}) bool {
		return f(key.(CommandNode), value.(string))
	})
}

func (m *commandNodeStringMap) Put(key CommandNode, value string) { m.Map.Put(key, value) }
func (m *commandNodeStringMap) Get(key CommandNode) (string, bool) {
	v, found := m.Map.Get(key)
	if found {
		return v.(string), true
	}
	return "", false
}
func (m *commandNodeStringMap) Remove(key CommandNode) { m.Map.Remove(key) }
func (m *commandNodeStringMap) Keys() []CommandNode {
	keys := m.Map.Keys()
	a := make([]CommandNode, len(keys))
	for i, k := range keys {
		a[i] = k.(CommandNode)
	}
	return a
}
func (m *commandNodeStringMap) Values() []string {
	values := m.Map.Values()
	a := make([]string, len(values))
	for i, v := range values {
		a[i] = v.(string)
	}
	return a
}

type stringCommandNodeMap struct{ *linkedhashmap.Map }

func (m *stringCommandNodeMap) MarshalJSON() ([]byte, error)    { return m.Map.ToJSON() }
func (m *stringCommandNodeMap) UnmarshalJSON(data []byte) error { return m.Map.FromJSON(data) }

var _ StringCommandNodeMap = (*stringCommandNodeMap)(nil)
var _ json.Marshaler = (*stringCommandNodeMap)(nil)
var _ json.Unmarshaler = (*stringCommandNodeMap)(nil)

func (m *stringCommandNodeMap) Range(f func(key string, value CommandNode) bool) {
	m.Map.All(func(key interface{}, value interface{}) bool {
		return f(key.(string), value.(CommandNode))
	})
}

func (m *stringCommandNodeMap) Put(key string, value CommandNode) { m.Map.Put(key, value) }
func (m *stringCommandNodeMap) Get(key string) (CommandNode, bool) {
	v, found := m.Map.Get(key)
	if found {
		return v.(CommandNode), true
	}
	return nil, false
}
func (m *stringCommandNodeMap) Remove(key string) { m.Map.Remove(key) }
func (m *stringCommandNodeMap) Keys() []string {
	keys := m.Map.Keys()
	a := make([]string, len(keys))
	for i, k := range keys {
		a[i] = k.(string)
	}
	return a
}
func (m *stringCommandNodeMap) Values() []CommandNode {
	values := m.Map.Values()
	a := make([]CommandNode, len(values))
	for i, v := range values {
		a[i] = v.(CommandNode)
	}
	return a
}
