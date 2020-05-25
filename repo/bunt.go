package repo

import (
	"encoding/json"
	"github.com/tidwall/buntdb"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"os"
)

type buntRepo struct {
	changes      chan Change
	db           *buntdb.DB
	startedEmpty bool
	generateId   IdGenerator

	attachments AttachmentRepo
	cases       CaseRepo
	logs        LogRepo
	suites      SuiteRepo
}

func OpenBuntRepos(filename string, generateId IdGenerator) (Repos, error) {
	if generateId == nil {
		generateId = uniqueIdGenerator
	}

	_, dbStatErr := os.Stat(filename)
	db, err := buntdb.Open(filename)
	if err != nil {
		return nil, err
	}
	r := &buntRepo{
		changes:      make(chan Change),
		db:           db,
		startedEmpty: os.IsNotExist(dbStatErr),
		generateId:   generateId,
	}
	if r.attachments, err = r.newAttachmentRepo(); err != nil {
		return nil, err
	}
	if r.cases, err = r.newCaseRepo(); err != nil {
		return nil, err
	}
	if r.logs, err = r.newLogRepo(); err != nil {
		return nil, err
	}
	if r.suites, err = r.newSuiteRepo(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *buntRepo) Attachments() AttachmentRepo {
	return r.attachments
}

func (r *buntRepo) Cases() CaseRepo {
	return r.cases
}

func (r *buntRepo) Changes() <-chan Change {
	return r.changes
}

func (r *buntRepo) Logs() LogRepo {
	return r.logs
}

func (r *buntRepo) StartedEmpty() bool {
	return r.startedEmpty
}

func (r *buntRepo) Suites() SuiteRepo {
	return r.suites
}

func (r *buntRepo) Close() error {
	if err := r.db.Close(); err != nil {
		return err
	}
	close(r.changes)
	return nil
}

func (r *buntRepo) save(coll Collection, e interface{}) (string, error) {
	b, err := json.Marshal(&e)
	if err != nil {
		return "", err
	}
	v := string(b)
	var id string
	err = r.db.Update(func(tx *buntdb.Tx) error {
		id = r.generateId()
		v, err := sjson.Set(v, "id", id)
		if err != nil {
			return err
		}
		_, _, err = tx.Set(string(coll)+":"+id, v, nil)
		if err != nil {
			return err
		}
		var payload interface{}
		if err := json.Unmarshal([]byte(v), &payload); err != nil {
			return err
		}
		r.changes <- Change{
			Op:      ChangeOpInsert,
			Coll:    coll,
			Payload: payload,
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *buntRepo) set(coll Collection, id string, m map[string]interface{}) error {
	k := string(coll) + ":" + id
	return r.db.Update(func(tx *buntdb.Tx) error {
		v, err := tx.Get(k)
		if err != nil {
			return err
		}
		for mK, mV := range m {
			if mV == nil {
				continue
			}
			if v, err = sjson.Set(v, mK, mV); err != nil {
				return err
			}
		}
		if _, _, err = tx.Set(k, v, nil); err != nil {
			return err
		}
		var payload interface{}
		if err := json.Unmarshal([]byte(v), &payload); err != nil {
			return err
		}
		r.changes <- Change{
			Op:      ChangeOpUpdate,
			Coll:    coll,
			Payload: payload,
		}
		return nil
	})
}

func (r *buntRepo) find(coll Collection, id string, e interface{}) error {
	k := string(coll) + ":" + id
	var v string
	err := r.db.View(func(tx *buntdb.Tx) error {
		var err error
		v, err = tx.Get(k)
		return err
	})
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(v), &e)
}

func (r *buntRepo) findAll(index string, includeDeleted bool, entities interface{}) error {
	var values []string
	iterator := func(k, v string) bool {
		values = append(values, v)
		return true
	}
	err := r.db.View(func(tx *buntdb.Tx) error {
		if includeDeleted {
			return tx.Ascend(index, iterator)
		}
		return tx.AscendEqual(index, `{"deleted":false}`, iterator)
	})
	if err != nil {
		return err
	}
	return jsonValuesToArr(values, &entities)
}

func (r *buntRepo) findAllBy(index string, m map[string]interface{}, entities interface{}) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	pivot := string(b)
	var values []string
	iterator := func(k, v string) bool {
		values = append(values, v)
		return true
	}
	err = r.db.View(func(tx *buntdb.Tx) error {
		return tx.AscendEqual(index, pivot, iterator)
	})
	if err != nil {
		return err
	}
	return jsonValuesToArr(values, &entities)
}

func (r *buntRepo) delete(coll Collection, id string, at int64) error {
	return r.db.Update(func(tx *buntdb.Tx) error {
		k := string(coll) + ":" + id
		v, err := tx.Get(k)
		if err != nil {
			return err
		}
		if v, err = sjson.Set(v, "deleted", true); err != nil {
			return err
		}
		if v, err = sjson.Set(v, "deleted_at", at); err != nil {
			return err
		}
		if _, _, err := tx.Set(k, v, nil); err != nil {
			return err
		}
		var payload interface{}
		if err := json.Unmarshal([]byte(v), &payload); err != nil {
			return err
		}
		r.changes <- Change{
			Op:      ChangeOpUpdate,
			Coll:    coll,
			Payload: payload,
		}
		return nil
	})
}

func (r *buntRepo) deleteAll(coll Collection, index string, at int64) error {
	entries := make(map[string]string)
	iterator := func(k, v string) bool {
		entries[k] = v
		return true
	}
	return r.db.Update(func(tx *buntdb.Tx) error {
		err := tx.AscendEqual(index, `{"deleted":false}`, iterator)
		if err != nil {
			return err
		}
		for k, v := range entries {
			if v, err = sjson.Set(v, "deleted", true); err != nil {
				return err
			}
			if v, err = sjson.Set(v, "deleted_at", at); err != nil {
				return err
			}
			if _, _, err := tx.Set(k, v, nil); err != nil {
				return err
			}
			var payload interface{}
			if err := json.Unmarshal([]byte(v), &payload); err != nil {
				return err
			}
			r.changes <- Change{
				Op:      ChangeOpUpdate,
				Coll:    coll,
				Payload: payload,
			}
		}
		return nil
	})
}

func indexJSONOptional(path string) func(a, b string) bool {
	return func(a, b string) bool {
		aResult := gjson.Get(a, path)
		bResult := gjson.Get(b, path)
		if aResult.Exists() && bResult.Exists() {
			return aResult.Less(bResult, false)
		}
		return false
	}
}
