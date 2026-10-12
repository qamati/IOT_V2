package rule

import (
	"encoding/json"
	"fmt"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// --- flow.output ---

type FlowOutputConfig struct {
	Relation string `json:"relation,omitempty"`
}

type FlowOutputNode struct {
	cfg FlowOutputConfig
}

func (n *FlowOutputNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &n.cfg)
	}
	if n.cfg.Relation == "" {
		n.cfg.Relation = RelationSuccess
	}
	return nil
}

func (n *FlowOutputNode) OnMsg(ctx NodeCtx, msg *Message) {
	ctx.TellNext(msg, n.cfg.Relation)
}

func (n *FlowOutputNode) Close() {}

// --- flow.rule_chain ---

type FlowRuleChainConfig struct {
	RuleChainID string `json:"ruleChainId"`
}

type FlowRuleChainNode struct {
	cfg        FlowRuleChainConfig
	targetChain ids.ID
}

func (n *FlowRuleChainNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		if err := json.Unmarshal(cfg, &n.cfg); err != nil {
			return fmt.Errorf("invalid flow.rule_chain config: %w", err)
		}
	}
	id, err := ids.Parse(n.cfg.RuleChainID)
	if err != nil {
		return fmt.Errorf("invalid target ruleChainId: %w", err)
	}
	n.targetChain = id
	return nil
}

func (n *FlowRuleChainNode) OnMsg(ctx NodeCtx, msg *Message) {
	outMsg := msg.Clone()
	outMsg.ChainID = n.targetChain
	ctx.TellNext(outMsg, RelationSuccess)
}

func (n *FlowRuleChainNode) Close() {}

func registerFlowNodes(c *Catalog) {
	c.Register(Descriptor{
		Type:      "flow.output",
		Name:      "Flow Output",
		Category:  "FLOW",
		Relations: []string{RelationSuccess},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"relation": {"type": "string", "default": "Success", "title": "Exit Relation Label"}
			}
		}`),
		Version: 1,
		Docs:    "Terminates current flow execution and emits specified relation label.",
	}, func() Node { return &FlowOutputNode{} })

	c.Register(Descriptor{
		Type:      "flow.rule_chain",
		Name:      "Rule Chain",
		Category:  "FLOW",
		Relations: []string{RelationSuccess, RelationFailure},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"ruleChainId": {"type": "string", "title": "Target Rule Chain UUID"}
			},
			"required": ["ruleChainId"]
		}`),
		Version: 1,
		Docs:    "Forwards message execution to a nested secondary rule chain.",
	}, func() Node { return &FlowRuleChainNode{} })
}
