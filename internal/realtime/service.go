package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/qamati/IOT_V2/internal/identity"
	"github.com/qamati/IOT_V2/internal/model"
	"github.com/qamati/IOT_V2/pkg/bus"
	"github.com/qamati/IOT_V2/pkg/ids"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for dev/embedded console
	},
}

// FrameType identifies WebSocket messaging payload intentions.
type FrameType string

const (
	FrameTypeAuth        FrameType = "AUTH"
	FrameTypeSubscribe   FrameType = "SUBSCRIBE"
	FrameTypeUnsubscribe FrameType = "UNSUBSCRIBE"
	FrameTypePing        FrameType = "PING"
	FrameTypePong        FrameType = "PONG"
	FrameTypeConnected   FrameType = "CONNECTED"
	FrameTypeUpdate      FrameType = "UPDATE"
	FrameTypeError       FrameType = "ERROR"
)

// Kind identifies real-time subscription channels.
type Kind string

const (
	KindTelemetry     Kind = "TELEMETRY"
	KindAlarms        Kind = "ALARMS"
	KindNotifications Kind = "NOTIFICATIONS"
	KindScada         Kind = "SCADA"
)

// ClientFrame represents an incoming request frame from browser client.
type ClientFrame struct {
	Type     FrameType `json:"type"`
	Token    string    `json:"token,omitempty"`
	CmdID    int       `json:"cmdId,omitempty"`
	Kind     Kind      `json:"kind,omitempty"`
	EntityID string    `json:"entityId,omitempty"`
	Keys     []string  `json:"keys,omitempty"`
}

// ServerFrame represents an outgoing message to the browser client.
type ServerFrame struct {
	Type      FrameType `json:"type"`
	SessionID string    `json:"sessionId,omitempty"`
	CmdID     int       `json:"cmdId,omitempty"`
	Kind      Kind      `json:"kind,omitempty"`
	EntityID  string    `json:"entityId,omitempty"`
	Data      any       `json:"data,omitempty"`
	Timestamp int64     `json:"timestamp,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Subscription tracks a client's interest in entity data streams.
type Subscription struct {
	CmdID    int
	Kind     Kind
	EntityID string
	Keys     map[string]bool
}

// Session manages an active WebSocket client connection.
type Session struct {
	id       string
	tenantID ids.ID
	userID   ids.ID
	conn     *websocket.Conn
	send     chan []byte
	subs     map[int]*Subscription // cmdId -> Subscription
	mu       sync.RWMutex
	service  *service
}

func (s *Session) addSubscription(sub *Subscription) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[sub.CmdID] = sub
}

func (s *Session) removeSubscription(cmdID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, cmdID)
}

func (s *Session) sendFrame(frame ServerFrame) {
	data, err := json.Marshal(frame)
	if err != nil {
		return
	}

	select {
	case s.send <- data:
	default:
		// Drop frame if slow consumer buffer is full
	}
}

// Service provides real-time WebSocket connection handling and event fan-out.
type Service interface {
	HandleWebSocket(w http.ResponseWriter, r *http.Request)
	BroadcastTelemetry(tenantID, deviceID ids.ID, ts int64, values map[string]any)
	BroadcastAlarm(a *model.Alarm)
	BroadcastNotification(n *model.Notification)
	Start(ctx context.Context, b bus.Bus) error
	Close() error
}

type service struct {
	idSvc    *identity.Service
	logger   *slog.Logger
	sessions map[string]*Session
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewService creates a new WebSocket Realtime Gateway service.
func NewService(idSvc *identity.Service, logger *slog.Logger) Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &service{
		idSvc:    idSvc,
		logger:   logger,
		sessions: make(map[string]*Session),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (s *service) registerSession(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.id] = sess
}

func (s *service) unregisterSession(sess *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sess.id)
	close(sess.send)
	_ = sess.conn.Close()
}

// HandleWebSocket upgrades HTTP requests to WebSocket sessions.
func (s *service) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("failed to upgrade websocket", "error", err)
		return
	}

	// Authenticate query param token if present
	var tenantID, userID ids.ID
	tokenStr := r.URL.Query().Get("token")
	if tokenStr != "" && s.idSvc != nil {
		claims, err := s.idSvc.ValidateAccessToken(tokenStr)
		if err == nil && claims != nil {
			tenantID, _ = ids.Parse(claims.TenantID)
			userID = claims.UserID
		}
	}

	sessionID := ids.MustNew().String()
	sess := &Session{
		id:       sessionID,
		tenantID: tenantID,
		userID:   userID,
		conn:     conn,
		send:     make(chan []byte, 256),
		subs:     make(map[int]*Subscription),
		service:  s,
	}

	s.registerSession(sess)
	s.logger.Debug("websocket client connected", "sessionId", sessionID, "tenantId", tenantID)

	// Send initial CONNECTED frame
	sess.sendFrame(ServerFrame{
		Type:      FrameTypeConnected,
		SessionID: sessionID,
		Timestamp: time.Now().UnixMilli(),
	})

	go sess.writePump()
	go sess.readPump()
}

func (s *Session) readPump() {
	defer s.service.unregisterSession(s)

	s.conn.SetReadLimit(65536)
	_ = s.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	s.conn.SetPongHandler(func(string) error {
		_ = s.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			break
		}

		var frame ClientFrame
		if err := json.Unmarshal(message, &frame); err != nil {
			s.sendFrame(ServerFrame{
				Type:  FrameTypeError,
				Error: "invalid json frame payload",
			})
			continue
		}

		s.handleClientFrame(frame)
	}
}

func (s *Session) writePump() {
	ticker := time.NewTicker(25 * time.Second)
	defer func() {
		ticker.Stop()
		_ = s.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-s.send:
			_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = s.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := s.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(msg)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Session) handleClientFrame(frame ClientFrame) {
	switch frame.Type {
	case FrameTypeAuth:
		if s.service.idSvc == nil {
			s.sendFrame(ServerFrame{Type: FrameTypeError, Error: "identity service unavailable"})
			return
		}
		claims, err := s.service.idSvc.ValidateAccessToken(frame.Token)
		if err != nil || claims == nil {
			s.sendFrame(ServerFrame{
				Type:  FrameTypeError,
				Error: "invalid or expired token",
			})
			return
		}
		s.tenantID, _ = ids.Parse(claims.TenantID)
		s.userID = claims.UserID
		s.sendFrame(ServerFrame{
			Type:      FrameTypeConnected,
			SessionID: s.id,
			Timestamp: time.Now().UnixMilli(),
		})

	case FrameTypePing:
		s.sendFrame(ServerFrame{
			Type:      FrameTypePong,
			Timestamp: time.Now().UnixMilli(),
		})

	case FrameTypeSubscribe:
		keySet := make(map[string]bool)
		for _, k := range frame.Keys {
			keySet[k] = true
		}
		sub := &Subscription{
			CmdID:    frame.CmdID,
			Kind:     frame.Kind,
			EntityID: frame.EntityID,
			Keys:     keySet,
		}
		s.addSubscription(sub)
		s.sendFrame(ServerFrame{
			Type:      FrameTypeUpdate,
			CmdID:     frame.CmdID,
			Kind:      frame.Kind,
			EntityID:  frame.EntityID,
			Timestamp: time.Now().UnixMilli(),
			Data:      map[string]string{"status": "SUBSCRIBED"},
		})

	case FrameTypeUnsubscribe:
		s.removeSubscription(frame.CmdID)
	}
}

// BroadcastTelemetry fans out telemetry updates to all sessions with active subscriptions.
func (s *service) BroadcastTelemetry(tenantID, deviceID ids.ID, ts int64, values map[string]any) {
	s.mu.RLock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.RUnlock()

	devIDStr := deviceID.String()

	for _, sess := range sessions {
		if sess.tenantID != ids.Nil && sess.tenantID != tenantID {
			continue // Tenant boundary isolation
		}

		sess.mu.RLock()
		subs := make([]*Subscription, 0, len(sess.subs))
		for _, sub := range sess.subs {
			if sub.Kind == KindTelemetry && (sub.EntityID == "" || sub.EntityID == devIDStr) {
				subs = append(subs, sub)
			}
		}
		sess.mu.RUnlock()

		for _, sub := range subs {
			// Filter keys if specific keys were requested
			var filtered map[string]any
			if len(sub.Keys) > 0 {
				filtered = make(map[string]any)
				for k, v := range values {
					if sub.Keys[k] {
						filtered[k] = v
					}
				}
			} else {
				filtered = values
			}

			if len(filtered) > 0 {
				sess.sendFrame(ServerFrame{
					Type:      FrameTypeUpdate,
					CmdID:     sub.CmdID,
					Kind:      KindTelemetry,
					EntityID:  devIDStr,
					Data:      filtered,
					Timestamp: ts,
				})
			}
		}
	}
}

// BroadcastAlarm fans out alarm updates to all active sessions in the tenant.
func (s *service) BroadcastAlarm(a *model.Alarm) {
	if a == nil {
		return
	}
	s.mu.RLock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.RUnlock()

	for _, sess := range sessions {
		if sess.tenantID != ids.Nil && sess.tenantID != a.TenantID {
			continue
		}

		sess.mu.RLock()
		subs := make([]*Subscription, 0, len(sess.subs))
		for _, sub := range sess.subs {
			if sub.Kind == KindAlarms {
				subs = append(subs, sub)
			}
		}
		sess.mu.RUnlock()

		for _, sub := range subs {
			sess.sendFrame(ServerFrame{
				Type:      FrameTypeUpdate,
				CmdID:     sub.CmdID,
				Kind:      KindAlarms,
				EntityID:  a.OriginatorID.String(),
				Data:      a,
				Timestamp: time.Now().UnixMilli(),
			})
		}
	}
}

// BroadcastNotification fans out notification alerts to relevant users in the tenant.
func (s *service) BroadcastNotification(n *model.Notification) {
	if n == nil {
		return
	}
	s.mu.RLock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.mu.RUnlock()

	for _, sess := range sessions {
		if sess.tenantID != ids.Nil && sess.tenantID != n.TenantID {
			continue
		}
		// Match specific recipient or tenant broadcast
		if n.RecipientID != ids.Nil && sess.userID != ids.Nil && n.RecipientID != sess.userID {
			continue
		}

		sess.mu.RLock()
		subs := make([]*Subscription, 0, len(sess.subs))
		for _, sub := range sess.subs {
			if sub.Kind == KindNotifications {
				subs = append(subs, sub)
			}
		}
		sess.mu.RUnlock()

		for _, sub := range subs {
			sess.sendFrame(ServerFrame{
				Type:      FrameTypeUpdate,
				CmdID:     sub.CmdID,
				Kind:      KindNotifications,
				Data:      n,
				Timestamp: n.CreatedTime,
			})
		}
	}
}

// Start attaches the Realtime service to the event bus to listen for events.
func (s *service) Start(ctx context.Context, b bus.Bus) error {
	if b == nil {
		return nil
	}

	// Subscribe to alarm events
	_, err := b.Subscribe(ctx, "alarm.events", "realtime-alarm-worker", func(c context.Context, msg *bus.Message) error {
		var a model.Alarm
		if err := json.Unmarshal(msg.Payload, &a); err == nil {
			s.BroadcastAlarm(&a)
		}
		return nil
	})
	if err != nil {
		s.logger.Warn("failed to subscribe realtime to alarm events", "error", err)
	}

	s.logger.Info("realtime websocket gateway event bus listener started")
	return nil
}

// Close gracefully terminates all active WebSocket sessions.
func (s *service) Close() error {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sess := range s.sessions {
		_ = sess.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutdown"))
		_ = sess.conn.Close()
	}
	s.sessions = make(map[string]*Session)
	return nil
}
