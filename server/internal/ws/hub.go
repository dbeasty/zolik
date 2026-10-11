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
	// Onlookers is what a socket that is not one of Messages' players is sent:
	// somebody watching the table. Peers write it to every local connection in
	// the game that Messages did not address.
	Onlookers interface{} `json:"onlookers,omitempty"`
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
	h.publish(gameID, messages, nil)
}

func (h *Hub) publish(gameID string, messages []PlayerMessage, onlookers interface{}) {
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
		Onlookers:      onlookers,
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
	// Whoever else has the table open — a link opened by somebody not seated —
	// is sent the board as they are entitled to see it, or they would be left
	// on the snapshot they were given when they arrived. build is asked for
	// their own id locally; peers get the unseated view (id "").
	seated := make(map[string]bool, len(recipients))
	for _, pid := range recipients {
		seated[pid] = true
	}
	for pid := range h.registry.ForGame(gameID) {
		if !seated[pid] {
			h.writeLocal(gameID, pid, build(pid))
		}
	}
	var onlookers interface{}
	if h.redis != nil {
		onlookers = build("")
	}
	h.publish(gameID, msgs, onlookers)
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
			addressed := make(map[string]bool, len(env.Messages))
			for _, m := range env.Messages {
				addressed[m.PlayerID] = true
				h.writeLocal(env.GameID, m.PlayerID, m.Payload)
			}
			if env.Onlookers != nil {
				for pid := range h.registry.ForGame(env.GameID) {
					if !addressed[pid] {
						h.writeLocal(env.GameID, pid, env.Onlookers)
					}
				}
			}
		}
	}
}

// WriteDirect sends to one local connection (e.g. WS connect handshake).
func (h *Hub) WriteDirect(gameID, playerID string, payload interface{}) {
	h.writeLocal(gameID, playerID, payload)
}
