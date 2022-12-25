package models

import (
	"encoding/base64"
	"fmt"
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
	v := fmt.Sprintf(`{"liver_id":%q}`, e.ID)
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func ref[T any](v T) *T {
	return &v
}
