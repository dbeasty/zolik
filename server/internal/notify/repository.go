package notify

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"zolik/server/internal/db"
)

// ErrNotFound is what every single-document read answers with when there is
// nothing there, on either engine.
var ErrNotFound = errors.New("notify: not found")

// Repository is the persistence behind profiles, circles and devices.
type Repository interface {
	GetProfile(ctx context.Context, key string) (Profile, error)
	GetProfileByFriendCode(ctx context.Context, code string) (Profile, error)
	PutProfile(ctx context.Context, p Profile) error
	// DeleteProfile removes a subject's profile, giving up the friend code it
	// held. Used when a profile moves to another key, which is the only way a
	// code can pass from one subject to another: a code names one person.
	DeleteProfile(ctx context.Context, key string) error

	GetEdge(ctx context.Context, owner, member string) (Edge, error)
	PutEdge(ctx context.Context, e Edge) error
	DeleteEdge(ctx context.Context, owner, member string) error
	EdgesByOwner(ctx context.Context, owner string) ([]Edge, error)
	EdgesByMember(ctx context.Context, member string) ([]Edge, error)

	PutDevice(ctx context.Context, d Device) error
	DeleteDevice(ctx context.Context, id string) error
	DevicesFor(ctx context.Context, key string) ([]Device, error)

	// Rekey moves everything held under one subject key to another — a guest
	// who signed in. Where both keys already hold the same thing (an edge to
	// the same person), the account's copy wins and the guest's is dropped.
	Rekey(ctx context.Context, from, to string) error
}

type mongoRepository struct {
	profiles, circle, devices *mongo.Collection
}

func NewRepository(m *db.Mongo) Repository {
	c := m.Collections()
	return &mongoRepository{profiles: c.NotifyProfiles, circle: c.NotifyCircle, devices: c.NotifyDevices}
}

var _ Repository = (*mongoRepository)(nil)

func notFound(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFound
	}
	return err
}

func (r *mongoRepository) GetProfile(ctx context.Context, key string) (Profile, error) {
	var p Profile
	return p, notFound(r.profiles.FindOne(ctx, bson.M{"_id": key}).Decode(&p))
}

func (r *mongoRepository) GetProfileByFriendCode(ctx context.Context, code string) (Profile, error) {
	var p Profile
	return p, notFound(r.profiles.FindOne(ctx, bson.M{"friendCode": code}).Decode(&p))
}

func (r *mongoRepository) PutProfile(ctx context.Context, p Profile) error {
	_, err := r.profiles.ReplaceOne(ctx, bson.M{"_id": p.Key}, p, options.Replace().SetUpsert(true))
	return err
}

func (r *mongoRepository) DeleteProfile(ctx context.Context, key string) error {
	_, err := r.profiles.DeleteOne(ctx, bson.M{"_id": key})
	return err
}

func (r *mongoRepository) GetEdge(ctx context.Context, owner, member string) (Edge, error) {
	var e Edge
	return e, notFound(r.circle.FindOne(ctx, bson.M{"_id": edgeID(owner, member)}).Decode(&e))
}

func (r *mongoRepository) PutEdge(ctx context.Context, e Edge) error {
	e.ID = edgeID(e.OwnerKey, e.MemberKey)
	_, err := r.circle.ReplaceOne(ctx, bson.M{"_id": e.ID}, e, options.Replace().SetUpsert(true))
	return err
}

func (r *mongoRepository) DeleteEdge(ctx context.Context, owner, member string) error {
	_, err := r.circle.DeleteOne(ctx, bson.M{"_id": edgeID(owner, member)})
	return err
}

func (r *mongoRepository) edges(ctx context.Context, filter bson.M) ([]Edge, error) {
	cur, err := r.circle.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var out []Edge
	return out, cur.All(ctx, &out)
}

func (r *mongoRepository) EdgesByOwner(ctx context.Context, owner string) ([]Edge, error) {
	return r.edges(ctx, bson.M{"ownerKey": owner})
}

func (r *mongoRepository) EdgesByMember(ctx context.Context, member string) ([]Edge, error) {
	return r.edges(ctx, bson.M{"memberKey": member})
}

func (r *mongoRepository) PutDevice(ctx context.Context, d Device) error {
	_, err := r.devices.ReplaceOne(ctx, bson.M{"_id": d.ID}, d, options.Replace().SetUpsert(true))
	return err
}

func (r *mongoRepository) DeleteDevice(ctx context.Context, id string) error {
	_, err := r.devices.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func (r *mongoRepository) DevicesFor(ctx context.Context, key string) ([]Device, error) {
	cur, err := r.devices.Find(ctx, bson.M{"subjectKey": key})
	if err != nil {
		return nil, err
	}
	var out []Device
	return out, cur.All(ctx, &out)
}

func (r *mongoRepository) Rekey(ctx context.Context, from, to string) error {
	return rekey(ctx, r, from, to)
}

// rekey is written once against the interface rather than per engine. It is
// a one-off per sign-in over a handful of rows, and the merge rules — the
// account's copy wins — are the part worth having in exactly one place.
func rekey(ctx context.Context, r Repository, from, to string) error {
	if from == "" || to == "" || from == to {
		return nil
	}
	if p, err := r.GetProfile(ctx, from); err == nil {
		if _, err := r.GetProfile(ctx, to); errors.Is(err, ErrNotFound) {
			// The profile moves rather than being copied: the friend code on
			// it names one person, and leaving the guest's profile behind
			// would leave two profiles claiming the same code — which the
			// unique index on Mongo and the code's reservation on KDB both
			// refuse, as they should.
			if err := r.DeleteProfile(ctx, from); err != nil {
				return err
			}
			p.Key = to
			p.UpdatedAt = time.Now().UTC()
			if err := r.PutProfile(ctx, p); err != nil {
				return err
			}
		}
	}
	move := func(e Edge) error {
		if err := r.DeleteEdge(ctx, e.OwnerKey, e.MemberKey); err != nil {
			return err
		}
		if e.OwnerKey == from {
			e.OwnerKey = to
		}
		if e.MemberKey == from {
			e.MemberKey = to
		}
		if e.OwnerKey == e.MemberKey {
			return nil // a guest and the account it became, in each other's circle
		}
		if _, err := r.GetEdge(ctx, e.OwnerKey, e.MemberKey); err == nil {
			return nil
		}
		return r.PutEdge(ctx, e)
	}
	owned, err := r.EdgesByOwner(ctx, from)
	if err != nil {
		return err
	}
	held, err := r.EdgesByMember(ctx, from)
	if err != nil {
		return err
	}
	for _, e := range append(owned, held...) {
		if err := move(e); err != nil {
			return err
		}
	}
	devices, err := r.DevicesFor(ctx, from)
	if err != nil {
		return err
	}
	for _, d := range devices {
		d.SubjectKey = to
		if err := r.PutDevice(ctx, d); err != nil {
			return err
		}
	}
	return nil
}
