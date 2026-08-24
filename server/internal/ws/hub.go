package ws

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const redisBroadcastChannel = "zolik:ws:broadcast"

// PlayerMessage is one JSON payload targeted at a single player connection.
type PlayerMessage struct {
	PlayerID string      `json:"playerId"`
	Payload  interface{} `json:"payload"`
}

type busEnvelope struct {
	GameID         string          `json:"gameId"`
	SourceInstance string          `json:"sourceInstance"`
	Messages       []PlayerMessage `json:"messages"`
}

// Hub routes WebSocket payloads to local connections and, when Redis is configured,
// to other server instances via pub/sub (game sync — not user/auth data).
type Hub struct {
	registry   *ConnRegistry
	redis      *redis.Client
	instanceID string

	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewHub(registry *ConnRegistry, redisURL string) (*Hub, error) {
	h := &Hub{
		registry:   registry,
		instanceID: os.Getenv("INSTANCE_ID"),
	}
	if h.instanceID == "" {
		h.instanceID = uuid.NewString()
	}

	if redisURL == "" {
		log.Printf("ws hub: instance %s (local-only, no REDIS_URL)", h.instanceID)
		return h, nil
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	h.redis = redis.NewClient(opt)

	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	if err := h.redis.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, err
	}
	go h.subscribe(ctx)
	log.Printf("ws hub: instance %s (redis pub/sub enabled)", h.instanceID)
	return h, nil
}

func (h *Hub) InstanceID() string {
	return h.instanceID
}

func (h *Hub) RedisEnabled() bool {
	return h.redis != nil
}

func (h *Hub) Close() error {
	h.mu.Lock()
	if h.cancel != nil {
		h.cancel()
	}
	h.mu.Unlock()
	if h.redis != nil {
		return h.redis.Close()
	}
	return nil
}

// Registry exposes the local connection registry (connect/disconnect).
func (h *Hub) Registry() *ConnRegistry {
	return h.registry
}

// Publish delivers messages to local sockets and fans out to peer instances.
func (h *Hub) Publish(gameID string, messages []PlayerMessage) {
	for _, m := range messages {
		h.writeLocal(gameID, m.PlayerID, m.Payload)
	}
	if h.redis == nil || len(messages) == 0 {
		return
	}
	env := busEnvelope{
		GameID:         gameID,
		SourceInstance: h.instanceID,
		Messages:       messages,
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return
	}
	if err := h.redis.Publish(context.Background(), redisBroadcastChannel, raw).Err(); err != nil {
		log.Printf("ws hub publish: %v", err)
	}
}

// BroadcastGameState sends personalised game_state to every recipient (local sockets + Redis peers).
// recipients must list all player IDs in the game so updates reach instances with no local connections.
func (h *Hub) BroadcastGameState(gameID string, recipients []string, build func(playerID string) interface{}) {
	if len(recipients) == 0 {
		return
	}
	msgs := make([]PlayerMessage, 0, len(recipients))
	for _, pid := range recipients {
		msgs = append(msgs, PlayerMessage{PlayerID: pid, Payload: build(pid)})
	}
	h.Publish(gameID, msgs)
}

func (h *Hub) writeLocal(gameID, playerID string, payload interface{}) {
	h.registry.WriteJSON(gameID, playerID, payload)
}

func (h *Hub) subscribe(ctx context.Context) {
	sub := h.redis.Subscribe(ctx, redisBroadcastChannel)
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			_ = sub.Close()
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var env busEnvelope
			if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
				continue
			}
			if env.SourceInstance == h.instanceID {
				continue
			}
			for _, m := range env.Messages {
				h.writeLocal(env.GameID, m.PlayerID, m.Payload)
			}
		}
	}
}

// WriteDirect sends to one local connection (e.g. WS connect handshake).
func (h *Hub) WriteDirect(gameID, playerID string, payload interface{}) {
	h.writeLocal(gameID, playerID, payload)
}
