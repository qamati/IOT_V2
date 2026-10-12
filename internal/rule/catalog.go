package rule

import (
	"fmt"
	"sync"
)

// Catalog maintains the registry of rule node descriptors and factories.
type Catalog struct {
	mu          sync.RWMutex
	descriptors map[string]Descriptor
	factories   map[string]func() Node
}

// NewCatalog creates an empty Catalog instance.
func NewCatalog() *Catalog {
	return &Catalog{
		descriptors: make(map[string]Descriptor),
		factories:   make(map[string]func() Node),
	}
}

// Register registers a node descriptor and factory constructor.
func (c *Catalog) Register(desc Descriptor, factory func() Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.descriptors[desc.Type] = desc
	c.factories[desc.Type] = factory
}

// GetDescriptor returns the descriptor for a specific node type.
func (c *Catalog) GetDescriptor(nodeType string) (Descriptor, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.descriptors[nodeType]
	return d, ok
}

// ListDescriptors returns all registered node descriptors.
func (c *Catalog) ListDescriptors() []Descriptor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	list := make([]Descriptor, 0, len(c.descriptors))
	for _, d := range c.descriptors {
		list = append(list, d)
	}
	return list
}

// NewNode instantiates a new Node instance by its type identifier.
func (c *Catalog) NewNode(nodeType string) (Node, error) {
	c.mu.RLock()
	factory, ok := c.factories[nodeType]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown rule node type: %q", nodeType)
	}
	return factory(), nil
}

// DefaultCatalog is the global registry populated with built-in standard nodes.
var DefaultCatalog = NewCatalog()

func init() {
	registerFilterNodes(DefaultCatalog)
	registerEnrichNodes(DefaultCatalog)
	registerActionNodes(DefaultCatalog)
	registerExternalNodes(DefaultCatalog)
	registerFlowNodes(DefaultCatalog)
}
