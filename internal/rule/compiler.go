package rule

import (
	"fmt"
	"github.com/qamati/IOT_V2/internal/model"
)

// ValidationError represents an issue discovered during chain compilation.
type ValidationError struct {
	NodeID  string `json:"nodeId,omitempty"`
	Message string `json:"message"`
	Level   string `json:"level"` // "ERROR" or "WARNING"
}

// CompiledChain holds the runtime graph of instantiated nodes and directed edges.
type CompiledChain struct {
	FirstNodeID string
	Nodes       map[string]Node
	NodeTypes   map[string]string
	NodeNames   map[string]string
	Edges       map[string]map[string][]string // fromNodeID -> relationLabel -> []toNodeID
	Env         Env
}

// Compiler builds and validates executable CompiledChain instances from ChainDefinitions.
type Compiler struct {
	catalog *Catalog
}

// NewCompiler creates a new Compiler with given node catalog.
func NewCompiler(catalog *Catalog) *Compiler {
	if catalog == nil {
		catalog = DefaultCatalog
	}
	return &Compiler{catalog: catalog}
}

// Compile compiles a rule chain definition into an executable graph.
func (c *Compiler) Compile(def *model.ChainDefinition, env Env) (*CompiledChain, []ValidationError, error) {
	var errs []ValidationError

	if def == nil || len(def.Nodes) == 0 {
		return nil, nil, fmt.Errorf("empty rule chain definition")
	}

	compiled := &CompiledChain{
		FirstNodeID: def.FirstNodeID,
		Nodes:       make(map[string]Node),
		NodeTypes:   make(map[string]string),
		NodeNames:   make(map[string]string),
		Edges:       make(map[string]map[string][]string),
		Env:         env,
	}

	nodeMap := make(map[string]model.RuleNode)
	for _, n := range def.Nodes {
		if _, exists := nodeMap[n.ID]; exists {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Message: fmt.Sprintf("duplicate node ID: %s", n.ID),
				Level:   "ERROR",
			})
			continue
		}
		nodeMap[n.ID] = n

		instance, err := c.catalog.NewNode(n.Type)
		if err != nil {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Message: fmt.Sprintf("unsupported node type %q: %v", n.Type, err),
				Level:   "ERROR",
			})
			continue
		}

		if err := instance.Init(n.Configuration, env); err != nil {
			errs = append(errs, ValidationError{
				NodeID:  n.ID,
				Message: fmt.Sprintf("node init failed: %v", err),
				Level:   "ERROR",
			})
			continue
		}

		compiled.Nodes[n.ID] = instance
		compiled.NodeTypes[n.ID] = n.Type
		compiled.NodeNames[n.ID] = n.Name
		compiled.Edges[n.ID] = make(map[string][]string)
	}

	// Validate FirstNodeID
	if def.FirstNodeID == "" {
		if len(def.Nodes) > 0 {
			compiled.FirstNodeID = def.Nodes[0].ID
			errs = append(errs, ValidationError{
				NodeID:  compiled.FirstNodeID,
				Message: "firstNodeId not specified, defaulting to first declared node",
				Level:   "WARNING",
			})
		}
	} else if _, ok := nodeMap[def.FirstNodeID]; !ok {
		errs = append(errs, ValidationError{
			NodeID:  def.FirstNodeID,
			Message: fmt.Sprintf("firstNodeId %q does not exist in nodes list", def.FirstNodeID),
			Level:   "ERROR",
		})
	}

	// Wire connections
	for _, conn := range def.Connections {
		if _, ok := nodeMap[conn.From]; !ok {
			errs = append(errs, ValidationError{
				NodeID:  conn.From,
				Message: fmt.Sprintf("connection from unknown node ID %q", conn.From),
				Level:   "ERROR",
			})
			continue
		}
		if _, ok := nodeMap[conn.To]; !ok {
			errs = append(errs, ValidationError{
				NodeID:  conn.To,
				Message: fmt.Sprintf("connection to unknown node ID %q", conn.To),
				Level:   "ERROR",
			})
			continue
		}

		label := conn.Label
		if label == "" {
			label = RelationSuccess
		}

		if compiled.Edges[conn.From] == nil {
			compiled.Edges[conn.From] = make(map[string][]string)
		}
		compiled.Edges[conn.From][label] = append(compiled.Edges[conn.From][label], conn.To)
	}

	// Check for critical compile errors
	for _, e := range errs {
		if e.Level == "ERROR" {
			return nil, errs, fmt.Errorf("rule chain compilation failed: %s (node: %s)", e.Message, e.NodeID)
		}
	}

	return compiled, errs, nil
}
