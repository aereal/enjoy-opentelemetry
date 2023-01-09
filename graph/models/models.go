package models

import (
	"encoding/base64"
	"encoding/json"
)

type Edge interface {
	Cursor() string
}

func hasNextPage[T Edge](edges []T, first int) bool {
	return len(edges) > first
}

func NewPageInfo[T Edge](edges []T, first int) *PageInfo {
	if len(edges) == 0 {
		return &PageInfo{}
	}
	hasNext := hasNextPage(edges, first)
	lastIdx := len(edges) - 1
	if hasNext {
		lastIdx = lastIdx - 1
	}
	return &PageInfo{
		HasNextPage: hasNext,
		StartCursor: ref(edges[0].Cursor()),
		EndCursor:   ref(edges[lastIdx].Cursor()),
	}
}

func NewEdges[T Edge](edges []T, first int) []T {
	if hasNextPage(edges, first) {
		return edges[:first]
	}
	return edges
}

type Liver struct {
	ID   uint64 `db:"liver_id"`
	Name string `json:"name" db:"name"`
	Age  *int   `json:"age" db:"age"`
}

type LiverEdge struct {
	*Liver
}

var _ Edge = &LiverEdge{}

func (e *LiverEdge) Cursor() string {
	return (&LiverCursor{LiverID: e.ID}).Encode()
}

func ref[T any](v T) *T {
	return &v
}

var (
	cursorEncoding   = base64.StdEncoding
	emptyLiverCursor = &LiverCursor{}
)

func EmptyLiverCursor() *LiverCursor {
	return emptyLiverCursor
}

func NewLiverCursorFrom(v string) (*LiverCursor, error) {
	b, err := cursorEncoding.DecodeString(v)
	if err != nil {
		return nil, err
	}
	var c LiverCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

type LiverCursor struct {
	LiverID uint64
}

func (c *LiverCursor) IsEmpty() bool {
	return c == nil || c == emptyLiverCursor
}

func (c *LiverCursor) Encode() string {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return cursorEncoding.EncodeToString(b)
}
