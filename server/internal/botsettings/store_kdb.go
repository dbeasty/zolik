package botsettings

import (
	"context"

	"zolik/server/internal/db"
)

// kdbStore is a read-modify-write of the one document inside the namespace's
// write lock, as every KDB repository here does in place of Mongo's update
// operators.
type kdbStore struct{ k *db.KDB }

// NewKDBStore builds the KDB-backed store.
func NewKDBStore(k *db.KDB) Store { return &kdbStore{k: k} }

func (s *kdbStore) HardModels(ctx context.Context) (map[string]HardModel, error) {
	raw, err := s.k.Get(db.NSSettings, DocID)
	if db.IsNotFound(err) {
		return map[string]HardModel{}, nil
	}
	if err != nil {
		return nil, err
	}
	var d Doc
	if err := db.UnmarshalDoc(raw, &d); err != nil {
		return nil, err
	}
	if d.HardModel == nil {
		d.HardModel = map[string]HardModel{}
	}
	return d.HardModel, nil
}

func (s *kdbStore) SetHardModel(ctx context.Context, game string, v HardModel) error {
	if err := checkGame(game); err != nil {
		return err
	}
	return s.k.Update(db.NSSettings, func(tx *db.Tx) error {
		d := Doc{ID: DocID}
		raw, err := tx.Get(DocID)
		switch {
		case err == nil:
			if err := db.UnmarshalDoc(raw, &d); err != nil {
				return err
			}
		case !db.IsNotFound(err):
			return err
		}
		if d.HardModel == nil {
			d.HardModel = map[string]HardModel{}
		}
		d.HardModel[game] = v
		doc, err := db.MarshalDoc(d)
		if err != nil {
			return err
		}
		return tx.Put(DocID, doc)
	})
}
