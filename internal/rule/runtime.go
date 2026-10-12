package rule

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// ExecutionContext manages the execution flow and trace collection for a single message.
type ExecutionContext struct {
	chain      *CompiledChain
	traceMode  bool
	traceMu    sync.Mutex
	trace      *ExecutionTrace
	hasFailure bool
	failureErr error
	logger     *slog.Logger
}

type nodeExecutionState struct {
	execCtx   *ExecutionContext
	nodeID    string
	nodeType  string
	inMsg     *Message
	startTime time.Time
	called    bool
	relations []string
	outMsg    *Message
	err       error
}

func (s *nodeExecutionState) TellNext(msg *Message, relations ...string) {
	s.called = true
	s.relations = relations
	s.outMsg = msg

	duration := float64(time.Since(s.startTime).Microseconds()) / 1000.0
	if s.execCtx.traceMode {
		s.execCtx.recordTrace(&NodeTraceRecord{
			NodeID:     s.nodeID,
			NodeType:   s.nodeType,
			InMessage:  s.inMsg.Clone(),
			OutMessage: msg.Clone(),
			Relations:  relations,
			DurationMs: duration,
			Timestamp:  time.Now().UnixMilli(),
		})
	}

	for _, rel := range relations {
		targets := s.execCtx.chain.Edges[s.nodeID][rel]
		if len(targets) == 0 && rel == RelationFailure {
			s.execCtx.hasFailure = true
			if s.err != nil {
				s.execCtx.failureErr = s.err
			} else {
				s.execCtx.failureErr = fmt.Errorf("node %s failed on relation Failure", s.nodeID)
			}
		}
		for _, targetID := range targets {
			s.execCtx.executeNode(targetID, msg)
		}
	}
}

func (s *nodeExecutionState) TellSuccess(msg *Message) {
	s.TellNext(msg, RelationSuccess)
}

func (s *nodeExecutionState) TellFailure(msg *Message, err error) {
	s.err = err
	s.called = true
	duration := float64(time.Since(s.startTime).Microseconds()) / 1000.0

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	if s.execCtx.traceMode {
		s.execCtx.recordTrace(&NodeTraceRecord{
			NodeID:     s.nodeID,
			NodeType:   s.nodeType,
			InMessage:  s.inMsg.Clone(),
			Relations:  []string{RelationFailure},
			Error:      errStr,
			DurationMs: duration,
			Timestamp:  time.Now().UnixMilli(),
		})
	}

	targets := s.execCtx.chain.Edges[s.nodeID][RelationFailure]
	if len(targets) == 0 {
		s.execCtx.hasFailure = true
		s.execCtx.failureErr = err
	} else {
		for _, targetID := range targets {
			s.execCtx.executeNode(targetID, msg)
		}
	}
}

func (s *nodeExecutionState) Services() *Services {
	if s.execCtx.chain.Env != nil {
		return s.execCtx.chain.Env.Services()
	}
	return nil
}

func (s *nodeExecutionState) Log() *slog.Logger {
	return s.execCtx.logger
}

func (s *nodeExecutionState) TenantID() ids.ID {
	if s.execCtx.chain.Env != nil {
		return s.execCtx.chain.Env.TenantID()
	}
	return ids.Nil
}

func (ec *ExecutionContext) recordTrace(r *NodeTraceRecord) {
	ec.traceMu.Lock()
	defer ec.traceMu.Unlock()
	if ec.trace != nil {
		ec.trace.Records = append(ec.trace.Records, r)
	}
}

func (ec *ExecutionContext) executeNode(nodeID string, msg *Message) {
	node, ok := ec.chain.Nodes[nodeID]
	if !ok {
		ec.logger.Warn("target node not found in compiled chain", "node_id", nodeID)
		return
	}

	// Hop limit / cycle protection
	if msg.Hops >= 100 {
		ec.hasFailure = true
		ec.failureErr = fmt.Errorf("execution exceeded maximum hops guard limit (100) at node %s", nodeID)
		if ec.traceMode {
			ec.recordTrace(&NodeTraceRecord{
				NodeID:    nodeID,
				NodeType:  ec.chain.NodeTypes[nodeID],
				InMessage: msg.Clone(),
				Error:     ec.failureErr.Error(),
				Timestamp: time.Now().UnixMilli(),
			})
		}
		return
	}

	nextMsg := msg.Clone()
	nextMsg.Hops++

	state := &nodeExecutionState{
		execCtx:   ec,
		nodeID:    nodeID,
		nodeType:  ec.chain.NodeTypes[nodeID],
		inMsg:     nextMsg,
		startTime: time.Now(),
	}

	defer func() {
		if r := recover(); r != nil {
			panicErr := fmt.Errorf("panic in rule node %s: %v", nodeID, r)
			ec.logger.Error("rule node panic recovered", "node_id", nodeID, "error", panicErr)
			state.TellFailure(nextMsg, panicErr)
		}
	}()

	node.OnMsg(state, nextMsg)

	// If node did not call any Tell*, auto-complete as leaf
	if !state.called {
		duration := float64(time.Since(state.startTime).Microseconds()) / 1000.0
		if ec.traceMode {
			ec.recordTrace(&NodeTraceRecord{
				NodeID:     nodeID,
				NodeType:   ec.chain.NodeTypes[nodeID],
				InMessage:  nextMsg.Clone(),
				DurationMs: duration,
				Timestamp:  time.Now().UnixMilli(),
			})
		}
	}
}

// Execute runs a message through the compiled rule chain.
func (c *CompiledChain) Execute(ctx context.Context, msg *Message, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	ec := &ExecutionContext{
		chain:     c,
		traceMode: false,
		logger:    logger,
	}

	if c.FirstNodeID == "" {
		return fmt.Errorf("no entry node in compiled chain")
	}

	ec.executeNode(c.FirstNodeID, msg)

	if ec.hasFailure {
		return ec.failureErr
	}
	return nil
}

// ExecuteWithTrace runs a message through the compiled rule chain and returns execution trace records.
func (c *CompiledChain) ExecuteWithTrace(ctx context.Context, msg *Message, logger *slog.Logger) (*ExecutionTrace, error) {
	if logger == nil {
		logger = slog.Default()
	}

	start := time.Now()
	trace := &ExecutionTrace{
		MessageID: msg.ID,
		ChainID:   msg.ChainID,
		Records:   make([]*NodeTraceRecord, 0),
	}

	ec := &ExecutionContext{
		chain:     c,
		traceMode: true,
		trace:     trace,
		logger:    logger,
	}

	if c.FirstNodeID == "" {
		trace.Success = false
		trace.Error = "no entry node in compiled chain"
		return trace, fmt.Errorf("%s", trace.Error)
	}

	ec.executeNode(c.FirstNodeID, msg)

	trace.TotalTimeMs = float64(time.Since(start).Microseconds()) / 1000.0
	trace.Success = !ec.hasFailure
	if ec.hasFailure && ec.failureErr != nil {
		trace.Error = ec.failureErr.Error()
		return trace, ec.failureErr
	}

	return trace, nil
}
