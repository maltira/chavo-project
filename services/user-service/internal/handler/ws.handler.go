package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ws "github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/utils"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/websocket"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSHandler управляет WebSocket-соединениями.
type WSHandler struct {
	rdb    *redis.Client
	status *utils.StatusManager
	log    *zap.Logger
}

func NewWSHandler(rdb *redis.Client, status *utils.StatusManager, log *zap.Logger) *WSHandler {
	return &WSHandler{rdb: rdb, status: status, log: log}
}

// Connect обновляет HTTP-соединение до WebSocket и регистрирует клиента.
func (h *WSHandler) Connect(c *gin.Context) {
	userID, err := uuid.Parse(c.GetHeader("X-User-ID"))
	if err != nil {
		respondError(c, apperror.ErrInvalidUUID, h.log)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Error("WS upgrade failed", zap.String("user_id", userID.String()), zap.Error(err))
		return
	}

	client := &websocket.Client{
		UserID: userID,
		Conn:   conn,
		Done:   make(chan struct{}),
	}

	websocket.ClientsMu.Lock()
	websocket.Clients[userID] = client
	websocket.ClientsMu.Unlock()

	h.status.SetOnline(userID)

	go h.writePump(client)
	go h.readPump(client)
	h.log.Info("User connected via WebSocket", zap.String("user_id", userID.String()))
}

// — Подписки на события Redis Pub/Sub —

func (h *WSHandler) PubSubBlock(ctx context.Context) {
	pubsub := h.rdb.Subscribe(ctx, "user:block:events")
	defer func() { _ = pubsub.Close() }()

	for msg := range pubsub.Channel() {
		var event utils.BlockEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			h.log.Error("Invalid block event", zap.Error(err))
			continue
		}

		blockedUserID, err := uuid.Parse(event.BlockedID)
		if err != nil {
			h.log.Error("Invalid UUID in block event", zap.String("blocked_id", event.BlockedID))
			continue
		}

		websocket.ClientsMu.RLock()
		if blockedClient, exists := websocket.Clients[blockedUserID]; exists && blockedClient.Conn != nil {
			blockedClient.Mu.Lock()
			err = blockedClient.Conn.WriteMessage(ws.TextMessage, []byte(msg.Payload))
			blockedClient.Mu.Unlock()
			if err != nil {
				h.log.Error("Failed to send block event", zap.String("user_id", event.BlockedID), zap.Error(err))
			}
		}
		websocket.ClientsMu.RUnlock()
	}
}

func (h *WSHandler) PubSubStatus(ctx context.Context) {
	pubsub := h.rdb.Subscribe(ctx, "user:status:events")
	defer func() { _ = pubsub.Close() }()

	for msg := range pubsub.Channel() {
		var event utils.StatusEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			continue
		}

		websocket.ClientsMu.RLock()
		for _, client := range websocket.Clients {
			if client.Conn == nil {
				continue
			}
			client.Mu.Lock()
			err := client.Conn.WriteMessage(ws.TextMessage, []byte(msg.Payload))
			if err != nil {
				h.log.Error("Failed to send status event", zap.String("user_id", client.UserID.String()), zap.Error(err))
			}
			client.Mu.Unlock()
		}
		websocket.ClientsMu.RUnlock()
	}
}

func (h *WSHandler) PubSubNewMessage(ctx context.Context) {
	pubsub := h.rdb.Subscribe(ctx, "nj")
	defer func() { _ = pubsub.Close() }()

	type MessageEvent struct {
		EventType    string   `json:"event_type"`
		ID           string   `json:"id"`
		ChatID       string   `json:"chat_id"`
		UserID       string   `json:"user_id"`
		Content      string   `json:"content"`
		Type         string   `json:"type"`
		Participants []string `json:"participants"`
	}

	for msg := range pubsub.Channel() {
		var event MessageEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			continue
		}

		participantUUIDs := make([]uuid.UUID, 0, len(event.Participants))
		for _, pID := range event.Participants {
			pUUID, err := uuid.Parse(pID)
			if err != nil {
				h.log.Warn("Invalid participant UUID", zap.String("id", pID))
				continue
			}
			participantUUIDs = append(participantUUIDs, pUUID)
		}

		websocket.ClientsMu.RLock()
		for _, pUUID := range participantUUIDs {
			if client, exists := websocket.Clients[pUUID]; exists && client.Conn != nil {
				client.Mu.Lock()
				err := client.Conn.WriteMessage(ws.TextMessage, []byte(msg.Payload))
				client.Mu.Unlock()
				if err != nil {
					h.log.Error("Failed to send message event", zap.String("user_id", pUUID.String()), zap.Error(err))
				}
			}
		}
		websocket.ClientsMu.RUnlock()
	}
}

func (h *WSHandler) PubSubReadAck(ctx context.Context) {
	pubsub := h.rdb.Subscribe(ctx, "chat:read_ack:events")
	defer func() { _ = pubsub.Close() }()

	for msg := range pubsub.Channel() {
		var event struct {
			Event        string   `json:"event"`
			ChatID       string   `json:"chat_id"`
			UserID       string   `json:"user_id"`
			Participants []string `json:"participants"`
			MessageIDs   []string `json:"message_ids"`
		}
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			h.log.Error("Invalid read_ack event", zap.Error(err))
			continue
		}

		broadcast, _ := json.Marshal(map[string]any{
			"event_type":  "read_update",
			"chat_id":     event.ChatID,
			"user_id":     event.UserID,
			"message_ids": event.MessageIDs,
		})

		recipientUUIDs := make([]uuid.UUID, 0, len(event.Participants))
		for _, p := range event.Participants {
			if p == event.UserID {
				continue
			}
			clientID, err := uuid.Parse(p)
			if err != nil {
				continue
			}
			recipientUUIDs = append(recipientUUIDs, clientID)
		}

		websocket.ClientsMu.RLock()
		for _, clientID := range recipientUUIDs {
			if client, exists := websocket.Clients[clientID]; exists && client.Conn != nil {
				client.Mu.Lock()
				err := client.Conn.WriteMessage(ws.TextMessage, broadcast)
				client.Mu.Unlock()
				if err != nil {
					h.log.Error("Failed to send read_update", zap.String("user_id", clientID.String()), zap.Error(err))
				}
			}
		}
		websocket.ClientsMu.RUnlock()
	}
}

func (h *WSHandler) readPump(c *websocket.Client) {
	defer func() {
		close(c.Done)
		websocket.ClientsMu.Lock()
		delete(websocket.Clients, c.UserID)
		websocket.ClientsMu.Unlock()
		h.status.SetOffline(c.UserID)
		_ = c.Conn.Close()
		h.log.Info("User disconnected", zap.String("user_id", c.UserID.String()))
	}()

	_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		_ = c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		h.status.SetOnline(c.UserID)
		return nil
	})
	c.Conn.SetReadLimit(512)

	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseGoingAway, ws.CloseAbnormalClosure) {
				h.log.Warn("Unexpected WS close", zap.String("user_id", c.UserID.String()), zap.Error(err))
			}
			break
		}

		var incoming struct {
			Type string `json:"type"`
		}
		if err = json.Unmarshal(msg, &incoming); err != nil {
			h.log.Warn("Invalid JSON from WS client", zap.String("user_id", c.UserID.String()), zap.Error(err))
			continue
		}

		switch incoming.Type {
		case "read":
			var readEvent struct {
				ChatID     string   `json:"chat_id"`
				MessageIDs []string `json:"message_ids"`
			}
			if err = json.Unmarshal(msg, &readEvent); err != nil {
				continue
			}
			chatID, err := uuid.Parse(readEvent.ChatID)
			if err != nil {
				continue
			}
			event := map[string]any{
				"event":       "read_update",
				"user_id":     c.UserID.String(),
				"chat_id":     chatID.String(),
				"message_ids": readEvent.MessageIDs,
			}
			payload, _ := json.Marshal(event)

			// Публикация в Redis Stream
			_, err = h.rdb.XAdd(context.Background(), &redis.XAddArgs{
				Stream: "chat.events",
				Values: map[string]any{
					"payload": string(payload),
				},
			}).Result()
			if err != nil {
				h.log.Error("Failed to publish read_update to Redis Stream", zap.Error(err))
			}
		}
	}
}

func (h *WSHandler) writePump(c *websocket.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Done:
			return
		case <-ticker.C:
			c.Mu.Lock()
			err := c.Conn.WriteControl(ws.PingMessage, []byte("ping"), time.Now().Add(10*time.Second))
			c.Mu.Unlock()
			if err != nil {
				h.log.Warn("Ping failed", zap.String("user_id", c.UserID.String()), zap.Error(err))
				return
			}
		}
	}
}
