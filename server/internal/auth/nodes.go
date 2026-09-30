package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// The kinds of node the cloud will enrol. A phone hosting a table and a
// server in a club house differ in what they are trusted to do later, so the
// kind is recorded at enrolment rather than guessed at from behaviour, and an
// unrecognised one is refused instead of stored for something else to puzzle
// over.
const (
	NodeKindPhone = "phone"
	NodeKindLAN   = "lan"
)

// nodeCredentialTTL is a year. A node credential is not a session: it is how
// a replica proves which replica it is when it syncs, and a phone that is
// opened twice a season must not have to re-enrol to do it. Revocation is by
// the record, not by the clock, which is why the credential can afford to
// live this long.
const nodeCredentialTTL = 365 * 24 * time.Hour

// Node is one enrolled replica: which install it is, whose it is, and the key
// it will sign with.
//
// The id is derived from the public key (see NodeIDFor), so the record is
// keyed by something the node can prove it holds rather than by a name it
// asked for. InstanceID is the install's own id, kept because it is what the
// person sees in the app and what a support conversation will be about.
type Node struct {
	// The bson names matter: a stored document's own "id" field is the
	// engine's document id, so a model that used that name for something of
	// its own would have it replaced by a UUID it never chose.
	ID         string    `bson:"_id" json:"id"`
	OwnerID    string    `bson:"ownerId" json:"ownerId"`
	Kind       string    `bson:"kind" json:"kind"`
	InstanceID string    `bson:"instanceId,omitempty" json:"instanceId,omitempty"`
	PublicKey  string    `bson:"publicKey" json:"publicKey"`
	EnrolledAt time.Time `bson:"enrolledAt" json:"enrolledAt"`
	LastSeenAt time.Time `bson:"lastSeenAt,omitempty" json:"lastSeenAt,omitempty"`
}

// NodeRepository is the persistence behind enrolled nodes, in the shape the
// rest of this package needs it.
//
// TODO: the KDB-backed implementation belongs in the "nodes" namespace, one
// document per node keyed by Node.ID, with a secondary read by OwnerID for
// "which of my devices are enrolled". It is deliberately not written here:
// the database layer is being reworked alongside this, and the in-memory
// implementation below is enough to serve the endpoint and to test it. Until
// the real one is wired in, a restart forgets every enrolment and a node has
// to enrol again, which is a visible failure rather than a silent one.
type NodeRepository interface {
	// SaveNode records an enrolment, replacing any earlier one for the same
	// node.
	SaveNode(ctx context.Context, n Node) error
	// FindNode returns one node, or ErrNotFound.
	FindNode(ctx context.Context, id string) (Node, error)
	// NodesForOwner lists an account's nodes, oldest enrolment first.
	NodesForOwner(ctx context.Context, ownerID string) ([]Node, error)
}

// MemoryNodeRepository is the stand-in NodeRepository.
type MemoryNodeRepository struct {
	mu    sync.Mutex
	nodes map[string]Node
}

func NewMemoryNodeRepository() *MemoryNodeRepository {
	return &MemoryNodeRepository{nodes: map[string]Node{}}
}

var _ NodeRepository = (*MemoryNodeRepository)(nil)

func (r *MemoryNodeRepository) SaveNode(_ context.Context, n Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[n.ID] = n
	return nil
}

func (r *MemoryNodeRepository) FindNode(_ context.Context, id string) (Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.nodes[id]
	if !ok {
		return Node{}, ErrNotFound
	}
	return n, nil
}

func (r *MemoryNodeRepository) NodesForOwner(_ context.Context, ownerID string) ([]Node, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Node
	for _, n := range r.nodes {
		if n.OwnerID == ownerID {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EnrolledAt.Before(out[j].EnrolledAt) })
	return out, nil
}

// NodeClaims is a node's own credential: the token it presents when it syncs
// its replica, as opposed to the access token it presents when it acts for
// whoever is playing on it.
//
// The subject is "node:<id>", prefixed for the same reason a seated account
// is "user:<hex>": one namespace of subjects, and no way for a node id and a
// player id to be mistaken for one another. Owner is the account that enrolled
// it, which is what the sync side will authorise against.
type NodeClaims struct {
	Owner string `json:"owner"`
	Kind  string `json:"kind"`
	jwt.RegisteredClaims
}

// NodeID is the id without its subject prefix.
func (c *NodeClaims) NodeID() string { return strings.TrimPrefix(c.Subject, "node:") }

// CreateNodeCredential signs the credential a node will authenticate its
// database sync with.
func CreateNodeCredential(nodeID, ownerID, kind string, ttl time.Duration) (string, error) {
	if nodeID == "" || ownerID == "" {
		return "", errors.New("node credential: needs a node and an owner")
	}
	signer := activeSigner()
	if signer == nil {
		return "", ErrNoSigningKey
	}
	if ttl <= 0 {
		ttl = nodeCredentialTTL
	}
	now := time.Now().UTC()
	return signEdDSA(signer, NodeClaims{
		Owner: ownerID,
		Kind:  kind,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "node:" + nodeID,
			Issuer:    tokenIssuer(),
			Audience:  jwt.ClaimStrings{AudienceNode},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	})
}

// VerifyNodeCredential checks a credential the way VerifyOfflinePass checks a
// pass, and refuses on the same terms: EdDSA only, a key this verifier holds,
// the node audience and nothing else, and an expiry that has to be there.
func VerifyNodeCredential(token string, keys PublicKeys) (*NodeClaims, error) {
	if keys == nil {
		return nil, errors.New("node credential: no keys to check against")
	}
	parsed, err := jwt.ParseWithClaims(strings.TrimSpace(token), &NodeClaims{},
		func(t *jwt.Token) (interface{}, error) {
			kid, _ := t.Header["kid"].(string)
			if kid == "" {
				return nil, errors.New("credential names no signing key")
			}
			pub, ok := keys.PublicKeyByID(kid)
			if !ok {
				return nil, fmt.Errorf("unknown signing key %q", kid)
			}
			return pub, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithAudience(AudienceNode),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*NodeClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("node credential: invalid")
	}
	if claims.NodeID() == "" || claims.NodeID() == claims.Subject {
		return nil, errors.New("node credential: subject is not a node")
	}
	if claims.Owner == "" {
		return nil, errors.New("node credential: names no owner")
	}
	return claims, nil
}

// VerifySeatReceiptFromNode is the cloud's side of a receipt: look up the
// node the receipt names, then check the receipt against the key that node
// enrolled with.
//
// The two steps are in that order and cannot be in any other. The id in an
// unverified token is a database lookup and nothing more, and it is the
// enrolled key found by that lookup which decides whether any of it is true.
// A node nobody has enrolled has no key here, so its receipts prove nothing,
// which is the point of enrolment.
func VerifySeatReceiptFromNode(ctx context.Context, nodes NodeRepository, token string) (*SeatReceiptClaims, Node, error) {
	if nodes == nil {
		return nil, Node{}, errors.New("seat receipt: no enrolled nodes to check against")
	}
	nodeID, err := PeekSeatReceiptNode(token)
	if err != nil {
		return nil, Node{}, err
	}
	node, err := nodes.FindNode(ctx, nodeID)
	if err != nil {
		return nil, Node{}, err
	}
	pub, err := parseEd25519Public(node.PublicKey)
	if err != nil {
		return nil, Node{}, fmt.Errorf("seat receipt: enrolled node %s has no usable key: %w", nodeID, err)
	}
	claims, err := VerifySeatReceipt(token, pub)
	if err != nil {
		return nil, Node{}, err
	}
	return claims, node, nil
}

type enrollNodeReq struct {
	// PubKey is the node's Ed25519 public key, base64url or hex. The cloud
	// never sees the private half, which is the point: enrolment hands out a
	// credential for a key the node already has rather than a secret the
	// cloud invented and now has to transport.
	PubKey string `json:"pubkey"`
	Kind   string `json:"kind"`
	// InstanceID is the install's own id, shown to the person in the app.
	InstanceID string `json:"instanceId,omitempty"`
}

// enrollNode registers a replica against the signed-in account and hands back
// the credential it will sync with.
//
// Authenticated as a real account, never as a guest: a guest is a device's
// play history and nothing to attach a replica to. A node already enrolled by
// somebody else is refused rather than taken over, because the id is derived
// from the key, so the only way to present a node that belongs to another
// account is to have that account's key.
func (h *Handlers) enrollNode(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	uc, _ := GetUserContext(req)
	if uc.IsGuest {
		http.Error(w, "sign in with an account first", http.StatusForbidden)
		return
	}
	if h.nodes == nil {
		http.Error(w, "this deployment does not enrol nodes", http.StatusServiceUnavailable)
		return
	}

	var body enrollNodeReq
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	pub, err := parseEd25519Public(body.PubKey)
	if err != nil {
		http.Error(w, "pubkey must be an Ed25519 public key", http.StatusBadRequest)
		return
	}
	kind := strings.ToLower(strings.TrimSpace(body.Kind))
	if kind != NodeKindPhone && kind != NodeKindLAN {
		http.Error(w, "kind must be "+NodeKindPhone+" or "+NodeKindLAN, http.StatusBadRequest)
		return
	}

	nodeID := NodeIDFor(pub)
	now := time.Now().UTC()
	node := Node{
		ID:         nodeID,
		OwnerID:    uc.UserID,
		Kind:       kind,
		InstanceID: trimTo(body.InstanceID, 64),
		PublicKey:  EncodeNodePublicKey(pub),
		EnrolledAt: now,
	}
	switch existing, err := h.nodes.FindNode(ctx, nodeID); {
	case err == nil && existing.OwnerID != uc.UserID:
		http.Error(w, "that node is enrolled to another account", http.StatusConflict)
		return
	case err == nil:
		// Re-enrolment by the owner is ordinary: an app reinstalled from a
		// backup, or a credential about to expire. The enrolment date is the
		// first one, so the record still says when this node appeared.
		node.EnrolledAt = existing.EnrolledAt
		node.LastSeenAt = now
	case errors.Is(err, ErrNotFound):
	default:
		internalError(w, "enrollNode", err)
		return
	}

	credential, err := CreateNodeCredential(nodeID, uc.UserID, kind, nodeCredentialTTL)
	if errors.Is(err, ErrNoSigningKey) {
		// Refused rather than fudged with a shared-secret token: a credential
		// nothing else can verify is worse than no credential, because the
		// node would believe it had enrolled.
		http.Error(w, "this deployment cannot issue node credentials", http.StatusServiceUnavailable)
		return
	}
	if err != nil {
		internalError(w, "enrollNode", err)
		return
	}
	if err := h.nodes.SaveNode(ctx, node); err != nil {
		internalError(w, "enrollNode", err)
		return
	}

	writeJSON(w, map[string]any{
		"nodeId":     nodeID,
		"credential": credential,
		"expiresAt":  now.Add(nodeCredentialTTL).Format(time.RFC3339),
		"kind":       kind,
	})
}

// listNodes tells an account which replicas it has enrolled, so the app can
// show them and, later, offer to retire one.
func (h *Handlers) listNodes(w http.ResponseWriter, req *http.Request) {
	uc, _ := GetUserContext(req)
	if uc.IsGuest {
		http.Error(w, "sign in with an account first", http.StatusForbidden)
		return
	}
	if h.nodes == nil {
		writeJSON(w, map[string]any{"nodes": []Node{}})
		return
	}
	nodes, err := h.nodes.NodesForOwner(req.Context(), uc.UserID)
	if err != nil {
		internalError(w, "listNodes", err)
		return
	}
	if nodes == nil {
		nodes = []Node{}
	}
	writeJSON(w, map[string]any{"nodes": nodes})
}

func trimTo(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n]
	}
	return s
}
