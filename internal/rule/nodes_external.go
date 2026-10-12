package rule

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type RestCallConfig struct {
	Method               string            `json:"method"`
	URL                  string            `json:"url"`
	Headers              map[string]string `json:"headers,omitempty"`
	TimeoutMs            int               `json:"timeoutMs,omitempty"`
	TreatNon2xxAsFailure bool              `json:"treatNon2xxAsFailure,omitempty"`
}

type RestCallNode struct {
	cfg        RestCallConfig
	httpClient *http.Client
}

func (n *RestCallNode) Init(cfg json.RawMessage, env Env) error {
	if len(cfg) > 0 {
		if err := json.Unmarshal(cfg, &n.cfg); err != nil {
			return fmt.Errorf("invalid external.rest_call config: %w", err)
		}
	}
	if n.cfg.Method == "" {
		n.cfg.Method = "POST"
	}
	if n.cfg.TimeoutMs <= 0 {
		n.cfg.TimeoutMs = 5000
	}
	timeout := time.Duration(n.cfg.TimeoutMs) * time.Millisecond
	n.httpClient = &http.Client{
		Timeout: timeout,
	}
	return nil
}

func (n *RestCallNode) OnMsg(ctx NodeCtx, msg *Message) {
	// Template substitution in URL
	targetURL := n.resolveTemplate(n.cfg.URL, msg)
	if err := validateSSRF(targetURL); err != nil {
		ctx.Log().Warn("external.rest_call blocked by SSRF guard", "url", targetURL, "error", err)
		ctx.TellFailure(msg, err)
		return
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), time.Duration(n.cfg.TimeoutMs)*time.Millisecond)
	defer cancel()

	var bodyReader io.Reader
	if len(msg.Data) > 0 && (n.cfg.Method == "POST" || n.cfg.Method == "PUT" || n.cfg.Method == "PATCH") {
		bodyReader = bytes.NewReader(msg.Data)
	}

	req, err := http.NewRequestWithContext(reqCtx, n.cfg.Method, targetURL, bodyReader)
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}

	for k, v := range n.cfg.Headers {
		req.Header.Set(k, n.resolveTemplate(v, msg))
	}
	if req.Header.Get("Content-Type") == "" && len(msg.Data) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		ctx.TellFailure(msg, err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	outMsg := msg.Clone()
	outMsg.Metadata["statusCode"] = fmt.Sprintf("%d", resp.StatusCode)
	outMsg.Metadata["statusText"] = resp.Status
	if len(respBody) > 0 {
		outMsg.Data = respBody
	}

	if n.cfg.TreatNon2xxAsFailure && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		ctx.TellNext(outMsg, RelationFailure)
		return
	}

	ctx.TellNext(outMsg, RelationSuccess)
}

func (n *RestCallNode) resolveTemplate(tmpl string, msg *Message) string {
	res := tmpl
	for k, v := range msg.Metadata {
		res = strings.ReplaceAll(res, "${"+k+"}", v)
	}
	res = strings.ReplaceAll(res, "${id}", msg.ID.String())
	res = strings.ReplaceAll(res, "${originator}", msg.Originator.ID.String())
	return res
}

func validateSSRF(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	hostname := strings.ToLower(parsed.Hostname())
	// Block AWS / GCP / Azure metadata endpoint
	if hostname == "169.254.169.254" || hostname == "metadata.google.internal" || hostname == "instance-data" {
		return fmt.Errorf("SSRF protection: prohibited cloud metadata destination %q", hostname)
	}
	return nil
}

func (n *RestCallNode) Close() {}

func registerExternalNodes(c *Catalog) {
	c.Register(Descriptor{
		Type:      "external.rest_call",
		Name:      "REST API Call",
		Category:  "EXTERNAL",
		Relations: []string{RelationSuccess, RelationFailure},
		ConfigSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"method": {"type": "string", "enum": ["GET", "POST", "PUT", "PATCH", "DELETE"], "default": "POST", "title": "HTTP Method"},
				"url": {"type": "string", "title": "Endpoint URL (supports ${variable})"},
				"headers": {"type": "object", "title": "HTTP Headers"},
				"timeoutMs": {"type": "integer", "default": 5000, "title": "Timeout (ms)"},
				"treatNon2xxAsFailure": {"type": "boolean", "default": true, "title": "Treat non-2xx as Failure"}
			},
			"required": ["url"]
		}`),
		Version: 1,
		Docs:    "Issues HTTP request to external third-party API or webhook with SSRF protection and dynamic template variable substitution.",
	}, func() Node { return &RestCallNode{} })
}
