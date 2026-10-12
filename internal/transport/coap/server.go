package coap

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/qamati/IOT_V2/internal/core"
	"github.com/qamati/IOT_V2/internal/devstate"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/internal/rule"
	"github.com/qamati/IOT_V2/internal/telemetry"
)

// CoAP Message Types (RFC 7252)
const (
	TypeConfirmable    byte = 0
	TypeNonConfirmable byte = 1
	TypeAcknowledgement byte = 2
	TypeReset          byte = 3
)

// CoAP Method / Response Codes (Class.Detail)
const (
	CodeGET      byte = 1  // 0.01
	CodePOST     byte = 2  // 0.02
	CodeChanged  byte = 68 // 2.04
	CodeNotFound byte = 132 // 4.04
	CodeInternal byte = 160 // 5.00
)

// CoAP Option Numbers
const (
	OptionUriPath       = 11
	OptionContentFormat = 12
)

// Message represents an RFC 7252 CoAP packet.
type Message struct {
	Type      byte
	Code      byte
	MessageID uint16
	Token     []byte
	Paths     []string
	Payload   []byte
}

// Server provides a zero-dependency RFC 7252 UDP CoAP transport.
type Server struct {
	addr       string
	devManager *core.DeviceManager
	telemetry  telemetry.Store
	attributes telemetry.AttributeStore
	tracker    devstate.Tracker
	ruleEngine *rule.Engine

	conn   *net.UDPConn
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer creates a CoAP UDP server.
func NewServer(
	addr string,
	devMgr *core.DeviceManager,
	ts telemetry.Store,
	attrs telemetry.AttributeStore,
	tracker devstate.Tracker,
	ruleEng *rule.Engine,
) *Server {
	return &Server{
		addr:       addr,
		devManager: devMgr,
		telemetry:  ts,
		attributes: attrs,
		tracker:    tracker,
		ruleEngine: ruleEng,
	}
}

// Start listens on UDP and begins handling CoAP packets.
func (s *Server) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)

	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return fmt.Errorf("coap: resolve addr failed: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("coap: listen failed: %w", err)
	}
	s.conn = conn

	s.wg.Add(1)
	go s.readLoop()

	return nil
}

// Stop shuts down the CoAP listener.
func (s *Server) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	s.wg.Wait()
}

func (s *Server) readLoop() {
	defer s.wg.Done()
	buf := make([]byte, 2048)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		n, remoteAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}

		packet := make([]byte, n)
		copy(packet, buf[:n])
		go s.handlePacket(packet, remoteAddr)
	}
}

func (s *Server) handlePacket(packet []byte, remoteAddr *net.UDPAddr) {
	msg, err := ParseMessage(packet)
	if err != nil {
		return
	}

	// We route /api/v1/{token}/{telemetry|attributes|rpc}
	// Paths slice: ["api", "v1", "{token}", "telemetry"]
	if len(msg.Paths) < 4 || msg.Paths[0] != "api" || msg.Paths[1] != "v1" {
		s.sendResponse(remoteAddr, msg, CodeNotFound, []byte("Not Found"))
		return
	}

	token := msg.Paths[2]
	resource := msg.Paths[3]

	// Lookup device by credentials token
	creds, err := s.devManager.AuthenticateToken(s.ctx, token)
	if err != nil || creds == nil {
		s.sendResponse(remoteAddr, msg, CodeNotFound, []byte("Device Unauthorized"))
		return
	}
	dev, err := s.devManager.GetDevice(s.ctx, creds.TenantID, creds.DeviceID)
	if err != nil || dev == nil {
		s.sendResponse(remoteAddr, msg, CodeNotFound, []byte("Device Not Found"))
		return
	}

	switch resource {
	case "telemetry":
		var rawMap map[string]any
		if err := json.Unmarshal(msg.Payload, &rawMap); err != nil {
			s.sendResponse(remoteAddr, msg, CodeInternal, []byte("Malformed JSON"))
			return
		}

		now := time.Now().UnixMilli()
		var points []*model.TelemetryDataPoint
		for k, v := range rawMap {
			pt := &model.TelemetryDataPoint{Key: k, Timestamp: now}
			switch num := v.(type) {
			case float64:
				pt.DblV = &num
			case string:
				pt.StrV = &num
			case bool:
				pt.BoolV = &num
			}
			points = append(points, pt)
		}

		if len(points) > 0 && s.telemetry != nil {
			_ = s.telemetry.Save(s.ctx, dev.ID, points)
		}
		if s.tracker != nil {
			s.tracker.RecordActivity(s.ctx, dev.ID, now)
		}
		if s.ruleEngine != nil && len(points) > 0 {
			s.ruleEngine.EvaluateTelemetry(s.ctx, dev.TenantID, dev.ID, points)
		}

		s.sendResponse(remoteAddr, msg, CodeChanged, []byte("OK"))

	case "attributes":
		var attrs map[string]any
		if err := json.Unmarshal(msg.Payload, &attrs); err != nil {
			s.sendResponse(remoteAddr, msg, CodeInternal, []byte("Malformed JSON"))
			return
		}
		if s.attributes != nil {
			_ = s.attributes.SaveAttributes(s.ctx, dev.ID, model.ScopeClient, attrs)
		}
		s.sendResponse(remoteAddr, msg, CodeChanged, []byte("OK"))

	default:
		s.sendResponse(remoteAddr, msg, CodeChanged, []byte("OK"))
	}
}

func (s *Server) sendResponse(remoteAddr *net.UDPAddr, req *Message, code byte, payload []byte) {
	// If Non-confirmable and not requested, skip unless error
	respType := TypeAcknowledgement
	if req.Type == TypeNonConfirmable {
		respType = TypeNonConfirmable
	}

	resp := &Message{
		Type:      respType,
		Code:      code,
		MessageID: req.MessageID,
		Token:     req.Token,
		Payload:   payload,
	}

	raw := EncodeMessage(resp)
	_, _ = s.conn.WriteToUDP(raw, remoteAddr)
}

// ParseMessage deserializes a raw CoAP packet.
func ParseMessage(b []byte) (*Message, error) {
	if len(b) < 4 {
		return nil, errors.New("packet too short")
	}

	version := (b[0] >> 6) & 0x03
	if version != 1 {
		return nil, errors.New("unsupported coap version")
	}

	msgType := (b[0] >> 4) & 0x03
	tkl := b[0] & 0x0F
	code := b[1]
	msgID := binary.BigEndian.Uint16(b[2:4])

	idx := 4
	if len(b) < idx+int(tkl) {
		return nil, errors.New("truncated token")
	}
	token := b[idx : idx+int(tkl)]
	idx += int(tkl)

	var paths []string
	var payload []byte
	currentOpt := 0

	for idx < len(b) {
		if b[idx] == 0xFF {
			idx++
			payload = b[idx:]
			break
		}

		optByte := b[idx]
		idx++

		delta := int((optByte >> 4) & 0x0F)
		length := int(optByte & 0x0F)

		if delta == 13 {
			if idx >= len(b) {
				break
			}
			delta = int(b[idx]) + 13
			idx++
		}
		if length == 13 {
			if idx >= len(b) {
				break
			}
			length = int(b[idx]) + 13
			idx++
		}

		currentOpt += delta
		if idx+length > len(b) {
			break
		}
		optVal := b[idx : idx+length]
		idx += length

		if currentOpt == OptionUriPath {
			paths = append(paths, string(optVal))
		}
	}

	return &Message{
		Type:      msgType,
		Code:      code,
		MessageID: msgID,
		Token:     token,
		Paths:     paths,
		Payload:   payload,
	}, nil
}

// EncodeMessage serializes a CoAP message into bytes.
func EncodeMessage(m *Message) []byte {
	var buf bytes.Buffer

	tkl := byte(len(m.Token) & 0x0F)
	b0 := (1 << 6) | ((m.Type & 0x03) << 4) | tkl
	buf.WriteByte(b0)
	buf.WriteByte(m.Code)

	var idBytes [2]byte
	binary.BigEndian.PutUint16(idBytes[:], m.MessageID)
	buf.Write(idBytes[:])

	if len(m.Token) > 0 {
		buf.Write(m.Token)
	}

	// Options
	lastOpt := 0
	for _, p := range m.Paths {
		delta := OptionUriPath - lastOpt
		length := len(p)

		var deltaNibble byte
		var extDelta []byte
		if delta < 13 {
			deltaNibble = byte(delta)
		} else {
			deltaNibble = 13
			extDelta = []byte{byte(delta - 13)}
		}

		var lenNibble byte
		var extLen []byte
		if length < 13 {
			lenNibble = byte(length)
		} else {
			lenNibble = 13
			extLen = []byte{byte(length - 13)}
		}

		optHeader := (deltaNibble << 4) | lenNibble
		buf.WriteByte(optHeader)
		if len(extDelta) > 0 {
			buf.Write(extDelta)
		}
		if len(extLen) > 0 {
			buf.Write(extLen)
		}
		buf.WriteString(p)
		lastOpt = OptionUriPath
	}

	// Payload marker
	if len(m.Payload) > 0 {
		buf.WriteByte(0xFF)
		buf.Write(m.Payload)
	}

	return buf.Bytes()
}
