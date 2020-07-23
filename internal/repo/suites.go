package repo

import (
	"fmt"
	"github.com/tidwall/buntdb"
	"github.com/tidwall/gjson"
)

type SuiteStatus string

const (
	SuiteStatusUnknown      SuiteStatus = "unknown"
	SuiteStatusStarted      SuiteStatus = "started"
	SuiteStatusFinished     SuiteStatus = "finished"
	SuiteStatusDisconnected SuiteStatus = "disconnected"
)

type SuiteResult string

const (
	SuiteResultUnknown SuiteResult = "unknown"
	SuiteResultPassed  SuiteResult = "passed"
	SuiteResultFailed  SuiteResult = "failed"
)

type Suite struct {
	Entity
	VersionedEntity
	SoftDeleteEntity
	Name           string      `json:"name"`
	Tags           []string    `json:"tags"`
	PlannedCases   int64       `json:"planned_cases"`
	Status         SuiteStatus `json:"status"`
	Result         SuiteResult `json:"result"`
	DisconnectedAt int64       `json:"disconnected_at"`
	StartedAt      int64       `json:"started_at"`
	FinishedAt     int64       `json:"finished_at"`
}

func (s *Suite) setId(id string) {
	s.Id = id
}

type SuiteAgg struct {
	VersionedEntity
	TotalCount   int64 `json:"total_count"`
	StartedCount int64 `json:"started_count"`
}

type SuitePage struct {
	SuiteAgg
	HasMore bool    `json:"has_more"`
	Suites  []Suite `json:"suites"`
}

func (r *Repo) InsertSuite(s Suite) (id string, err error) {
	return r.insertFunc(SuiteColl, &s, func(tx *buntdb.Tx) error {
		var agg SuiteAgg
		err := r.update(tx, SuiteAggColl, "", &agg, func() {
			agg.Version++
			agg.TotalCount++
			if s.Status == SuiteStatusStarted {
				agg.StartedCount++
			}
		})
		if err != nil {
			return err
		}
		r.pub.Publish(Changefeed{SuiteInsert{
			Suite: s,
			Agg:   agg,
		}})
		return nil
	})
}

func (r *Repo) Suite(id string) (Suite, error) {
	var s Suite
	if err := r.getById(SuiteColl, id, &s); err != nil {
		return Suite{}, err
	}
	return s, nil
}

func (r *Repo) SuitePage(fromId string, limit int) (SuitePage, error) {
	var page SuitePage
	var vals []string
	err := r.db.View(func(tx *buntdb.Tx) error {
		var err error
		if vals, page.HasMore, err = getSuites(tx, fromId, limit); err != nil {
			return err
		}
		if err = r.getById(SuiteAggColl, "", &page.SuiteAgg); err == ErrNotFound {
			err = nil
		}
		return err
	})
	if err == buntdb.ErrNotFound {
		return SuitePage{}, ErrNotFound
	} else if err != nil {
		return SuitePage{}, err
	}
	page.Suites = make([]Suite, len(vals))
	unmarshalJsonVals(vals, func(i int) interface{} {
		return &page.Suites[i]
	})
	return page, nil
}

func getSuites(tx *buntdb.Tx, fromId string, limit int) (vals []string, hasMore bool, err error) {
	var startedAt int64
	less := func(string) bool {
		return true
	}
	if fromId != "" {
		var err error
		v, err := tx.Get(key(SuiteColl, fromId))
		if err != nil {
			return nil, false, err
		}
		startedAt = gjson.Get(v, "started_at").Int()
		less = func(v string) bool {
			return gjson.Get(v, "started_at").Int() < startedAt
		}
	}
	itr := newPageItr(limit, &vals, &hasMore, less)
	err = descendSuites(tx, fromId, startedAt, itr)
	return
}

func descendSuites(tx *buntdb.Tx, fromId string, startedAt int64, itr itr) error {
	if fromId == "" {
		return tx.Descend(suiteIndexStartedAt, itr)
	}
	pivot := fmt.Sprintf(`{"started_at": %d}`, startedAt)
	return tx.DescendLessOrEqual(suiteIndexStartedAt, pivot, itr)
}
