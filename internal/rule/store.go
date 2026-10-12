package rule

import (
	"context"
	"encoding/json"

	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/ids"
)

// Store defines persistence methods for Rule Chains and their versioned graph revisions.
type Store interface {
	ListRuleChains(ctx context.Context, tenantID ids.ID, page, limit int) ([]*model.RuleChain, int, error)
	GetRuleChain(ctx context.Context, tenantID, id ids.ID) (*model.RuleChain, error)
	GetRootRuleChain(ctx context.Context, tenantID ids.ID) (*model.RuleChain, error)
	SaveRuleChain(ctx context.Context, chain *model.RuleChain) error
	SetRootRuleChain(ctx context.Context, tenantID, id ids.ID) error
	DeleteRuleChain(ctx context.Context, tenantID, id ids.ID) error

	GetRevision(ctx context.Context, chainID ids.ID, revision int) (*model.RuleChainRevision, error)
	GetLatestRevision(ctx context.Context, chainID ids.ID) (*model.RuleChainRevision, error)
	SaveRevision(ctx context.Context, rev *model.RuleChainRevision) error
}

// NewDefaultRootChainDefinition generates a ThingsBoard-compatible root chain graph.
// Nodes:
// 1. Message Type Switch (Entry)
// 2. Save Time-Series (Post telemetry -> Save)
// 3. Save Attributes (Post attributes -> Save)
// 4. Log Message (Other -> Log)
func NewDefaultRootChainDefinition() model.ChainDefinition {
	return model.ChainDefinition{
		FirstNodeID: "n1",
		Nodes: []model.RuleNode{
			{
				ID:            "n1",
				Type:          "filter.msg_type_switch",
				Name:          "Message Type Switch",
				Configuration: json.RawMessage(`{}`),
				AdditionalInfo: json.RawMessage(`{"layout":{"x":100,"y":200}}`),
			},
			{
				ID:            "n2",
				Type:          "action.save_timeseries",
				Name:          "Save Time Series",
				Configuration: json.RawMessage(`{"useServerTs":false}`),
				AdditionalInfo: json.RawMessage(`{"layout":{"x":450,"y":120}}`),
			},
			{
				ID:            "n3",
				Type:          "action.save_attributes",
				Name:          "Save Client/Server Attributes",
				Configuration: json.RawMessage(`{"scope":"SERVER_SCOPE"}`),
				AdditionalInfo: json.RawMessage(`{"layout":{"x":450,"y":240}}`),
			},
			{
				ID:            "n4",
				Type:          "action.log",
				Name:          "Log Unhandled Event",
				Configuration: json.RawMessage(`{"level":"INFO"}`),
				AdditionalInfo: json.RawMessage(`{"layout":{"x":450,"y":360}}`),
			},
		},
		Connections: []model.RuleConnection{
			{
				From:  "n1",
				To:    "n2",
				Label: RelationPostTelemetry,
			},
			{
				From:  "n1",
				To:    "n3",
				Label: RelationPostAttributes,
			},
			{
				From:  "n1",
				To:    "n4",
				Label: RelationOther,
			},
		},
	}
}
