package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	pingInterval = 5 * time.Second
	rttAlpha     = 0.3
	writeTimeout = 10 * time.Second
)

var (
	wsTracer = otel.Tracer("ws")
	wsMeter  = otel.Meter("ws")

	connectionsActive metric.Int64UpDownCounter
	connectionsTotal  metric.Int64Counter
)

func init() {
	var err error
	connectionsActive, err = wsMeter.Int64UpDownCounter("rapid_ws_connections_active")
	if err != nil {
		panic(err)
	}
	connectionsTotal, err = wsMeter.Int64Counter("rapid_ws_connections_total")
	if err != nil {
		panic(err)
	}
}

type Client struct {
	ID     uuid.UUID
	Hub    *Hub
	Send   chan []byte
	GameCh chan Envelope

	conn   *websocket.Conn
	rtt    time.Duration
	mu     sync.RWMutex
	logger *slog.Logger

	// pending pings keyed by ping_id
	pendingPings   map[string]time.Time
	pendingPingsMu sync.Mutex
}

func newClient(id uuid.UUID, conn *websocket.Conn, hub *Hub, logger *slog.Logger) *Client {
	return &Client{
		ID:           id,
		Hub:          hub,
		Send:         make(chan []byte, 256),
		GameCh:       make(chan Envelope, 16),
		conn:         conn,
		logger:       logger.With("user_id", id),
		pendingPings: make(map[string]time.Time),
	}
}

func (c *Client) GetRTT() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rtt
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.Hub.unregister <- c
		c.conn.CloseNow()
	}()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				c.logger.Debug("client disconnected normally")
			} else {
				c.logger.Debug("read error", "error", err)
				connectionsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "error")))
			}
			return
		}

		var env Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			c.logger.Debug("invalid message format", "error", err)
			continue
		}

		_, span := wsTracer.Start(ctx, "ws.handle."+env.Type,
			trace.WithAttributes(attribute.String("user.id", c.ID.String())))

		switch env.Type {
		case "pong":
			c.handlePong(env)
		case "answer":
			select {
			case c.GameCh <- env:
			default:
				c.logger.Warn("game channel full, dropping answer")
			}
		case "challenge", "challenge_resp":
			c.Hub.handleMessage(c, env)
		default:
			c.logger.Debug("unknown message type", "type", env.Type)
		}

		span.End()
	}
}

func (c *Client) writePump(ctx context.Context) {
	defer c.conn.CloseNow()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				c.conn.Close(websocket.StatusNormalClosure, "")
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				c.logger.Debug("write error", "error", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pingID := uuid.New().String()
			c.pendingPingsMu.Lock()
			c.pendingPings[pingID] = time.Now()
			c.pendingPingsMu.Unlock()

			data, err := NewEnvelope("ping", PingMsg{PingID: pingID})
			if err != nil {
				continue
			}
			select {
			case c.Send <- data:
			default:
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) handlePong(env Envelope) {
	var msg PongMsg
	if err := json.Unmarshal(env.Payload, &msg); err != nil {
		return
	}

	c.pendingPingsMu.Lock()
	sentAt, ok := c.pendingPings[msg.PingID]
	if ok {
		delete(c.pendingPings, msg.PingID)
	}
	c.pendingPingsMu.Unlock()

	if !ok {
		return
	}

	sample := time.Since(sentAt)
	c.mu.Lock()
	if c.rtt == 0 {
		c.rtt = sample
	} else {
		c.rtt = time.Duration(float64(sample)*rttAlpha + float64(c.rtt)*(1-rttAlpha))
	}
	rtt := c.rtt
	c.mu.Unlock()

	c.logger.Debug("rtt measured", "sample_ms", sample.Milliseconds(), "ewma_ms", rtt.Milliseconds())
}
