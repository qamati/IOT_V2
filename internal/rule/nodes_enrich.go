package rule

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/qamati/IOT_V2/internal/model"
)

type EnrichOriginatorAttributesConfig struct {
	Scope               string   `json:"scope,omitempty"` // SERVER_SCOPE, SHARED_SCOPE, CLIENT_SCOPE, or empty for all
	Keys                []string `json:"keys,omitempty"`
	Target              string   `json:"target,omitempty"` // "metadata" or "data"
	TellFailureIfAbsent bool     `json:"tellFailureIfAbsent,omitempty"`
}

type EnrichOriginatorAttributesNode struct {
	cfg EnrichOriginatorAttributesConfig
}

func (n *EnrichOriginatorAttributesNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		if err := json.Unmarshal(cfg, &n.cfg); err != nil {
			return fmt.Errorf("invalid enrich.originator_attributes config: %w", err)
		}
	}
	if n.cfg.Target == "" {
		n.cfg.Target = "metadata"
	}
	return nil
}

func (n *EnrichOriginatorAttributesNode) OnMsg(ctx NodeCtx, msg *Message) {
	if ctx.Services() == nil || ctx.Services().Attributes == nil {
		ctx.TellNext(msg, RelationFailure)
		return
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var attrs []model.AttributeKv
	var err error

	if n.cfg.Scope != "" {
		scope := model.AttributeScope(n.cfg.Scope)
		attrs, err = ctx.Services().Attributes.GetAttributes(reqCtx, msg.Originator.ID, scope, n.cfg.Keys)
	} else {
		attrs, err = ctx.Services().Attributes.GetAllAttributes(reqCtx, msg.Originator.ID)
	}

	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}

	if len(attrs) == 0 && n.cfg.TellFailureIfAbsent {
		ctx.TellNext(msg, RelationFailure)
		return
	}

	outMsg := msg.Clone()

	if n.cfg.Target == "data" {
		var dataMap map[string]any
		_ = json.Unmarshal(outMsg.Data, &dataMap)
		if dataMap == nil {
			dataMap = make(map[string]any)
		}
		for _, attr := range attrs {
			if len(n.cfg.Keys) == 0 || containsKey(n.cfg.Keys, attr.Key) {
				dataMap[attr.Key] = attr.Val
			}
		}
		if updated, err := json.Marshal(dataMap); err == nil {
			outMsg.Data = updated
		}
	} else {
		// Target metadata
		for _, attr := range attrs {
			if len(n.cfg.Keys) == 0 || containsKey(n.cfg.Keys, attr.Key) {
				outMsg.Metadata[attr.Key] = fmt.Sprintf("%v", attr.Val)
			}
		}
	}

	ctx.TellNext(outMsg, RelationSuccess)
}

func containsKey(keys []string, target string) bool {
	for _, k := range keys {
		if k == target {
			return true
		}
	}
	return false
}

func (n *EnrichOriginatorAttributesNode) Close() {}

func registerEnrichNodes(c *Catalog) {
	c.Register(Descriptor{
		Type:      "enrich.originator_attributes",
		Name:      "Originator Attributes",
		Category:  "ENRICHMENT",
		Relations: []string{RelationSuccess, RelationFailure},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"scope": {"type": "string", "enum": ["", "SERVER_SCOPE", "SHARED_SCOPE", "CLIENT_SCOPE"], "title": "Attribute Scope"},
				"keys": {"type": "array", "items": {"type": "string"}, "title": "Attribute Keys"},
				"target": {"type": "string", "enum": ["metadata", "data"], "default": "metadata", "title": "Target Destination"},
				"tellFailureIfAbsent": {"type": "boolean", "default": false, "title": "Fail if Absent"}
			}
		}`),
		Version: 1,
		Docs:    "Loads client, server, or shared attributes of the originator device and injects them into message metadata or payload.",
	}, func() Node { return &EnrichOriginatorAttributesNode{} })
}
