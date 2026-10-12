package model

import (
	"encoding/json"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// EntityRef identifies a platform entity by its type and UUIDv7 ID.
type EntityRef struct {
	EntityType string `json:"entity_type"` // DEVICE, ASSET, TENANT, CUSTOMER
	ID         ids.ID `json:"id"`
}

// RuleChain represents the metadata of a directed graph processing chain.
type RuleChain struct {
	ID                ids.ID          `json:"id"`
	CreatedTime       int64           `json:"created_time"`
	TenantID          ids.ID          `json:"tenant_id"`
	Name              string          `json:"name"`
	Type              string          `json:"type"` // CORE, EDGE
	IsRoot            bool            `json:"is_root"`
	PublishedRevision int             `json:"published_revision"`
	AdditionalInfo    json.RawMessage `json:"additional_info,omitempty"`
	Version           int             `json:"version"`
}

// RuleChainRevision stores an immutable snapshot of a rule chain's graph definition.
type RuleChainRevision struct {
	RuleChainID ids.ID          `json:"rule_chain_id"`
	Revision    int             `json:"revision"`
	Status      string          `json:"status"` // DRAFT, PUBLISHED, ARCHIVED
	Definition  ChainDefinition `json:"definition"`
	AuthorID    *ids.ID         `json:"author_id,omitempty"`
	Comment     string          `json:"comment,omitempty"`
	CreatedTime int64           `json:"created_time"`
}

// ChainDefinition conforms to ThingsBoard graph representation: entry node, nodes, and labelled relations.
type ChainDefinition struct {
	FirstNodeID string           `json:"firstNodeId"`
	Nodes       []RuleNode       `json:"nodes"`
	Connections []RuleConnection `json:"connections"`
}

// RuleNode represents a single vertex in the rule engine execution graph.
type RuleNode struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	Name           string          `json:"name"`
	Configuration  json.RawMessage `json:"configuration"`
	AdditionalInfo json.RawMessage `json:"additionalInfo,omitempty"`
}

// RuleConnection represents a directed edge from one node to another tagged with a relation label.
type RuleConnection struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}
