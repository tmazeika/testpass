package repo

import (
	"encoding/json"
	"errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strconv"
	"strings"
	"sync/atomic"
)

type Collection string

const (
	AttachmentColl Collection = "attachments"
	CaseColl       Collection = "cases"
	LogColl        Collection = "logs"
	SuiteColl      Collection = "suites"
)

type Repos interface {
	Attachments() AttachmentRepo
	Cases() CaseRepo
	Changes() <-chan Change
	Logs() LogRepo
	Suites() SuiteRepo
	StartedEmpty() bool
	Close() error
}

type SavedEntity struct {
	Id string `json:"id" bson:"_id,omitempty"`
}

type SoftDeleteEntity struct {
	Deleted   bool  `json:"deleted"`
	DeletedAt int64 `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

type IdGenerator func() string

var (
	IncIntIdGenerator IdGenerator = func() string {
		return strconv.FormatInt(atomic.AddInt64(&incIntId, 1), 10)
	}

	incIntId          int64       = 0
	uniqueIdGenerator IdGenerator = func() string {
		return primitive.NewObjectID().Hex()
	}
)

var (
	ErrExpired          = errors.New("expired")
	ErrNotFound         = errors.New("not found")
	ErrNotReconnectable = errors.New("not reconnectable")
)

func jsonValuesToArr(values []string, arr interface{}) error {
	v := "[" + strings.Join(values, ",") + "]"
	return json.Unmarshal([]byte(v), &arr)
}
