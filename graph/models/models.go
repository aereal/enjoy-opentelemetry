package models

import (
	"context"
	"encoding"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/aereal/enjoy-opentelemetry/domain"
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
	ID        uint64     `db:"liver_id"`
	Name      string     `json:"name" db:"name"`
	DebutedOn time.Time  `json:"debuted_on" db:"debuted_on"`
	RetiredOn *time.Time `json:"retired_on" db:"retired_on"`
}

func (l *Liver) Status() LiverStatus {
	if l.RetiredOn != nil {
		return LiverStatusRetired
	}
	if l.DebutedOn.After(time.Now()) {
		return LiverStatusAnnounced
	}
	return LiverStatusDebuted
}

func (l *Liver) EnrollmentDuration() time.Duration {
	to := time.Now()
	if l.RetiredOn != nil {
		to = *l.RetiredOn
	}
	return to.Sub(l.DebutedOn)
}

var aDay = time.Hour * 24

func (l *Liver) EnrollmentDays() int64 {
	return int64(l.EnrollmentDuration().Hours() / aDay.Hours())
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

const (
	LiverStatusAnnounced LiverStatus = iota + 1
	LiverStatusDebuted
	LiverStatusRetired

	valueAnnounced = "ANNOUNCED"
	valueDebuted   = "DEBUTED"
	valueRetired   = "RETIRED"
)

type LiverStatus int

var _ interface {
	fmt.Stringer
	encoding.TextMarshaler
	encoding.TextUnmarshaler
	graphql.ContextMarshaler
	graphql.ContextUnmarshaler
} = (*LiverStatus)(nil)

func (s LiverStatus) String() string {
	switch s {
	case LiverStatusAnnounced:
		return valueAnnounced
	case LiverStatusDebuted:
		return valueDebuted
	case LiverStatusRetired:
		return valueRetired
	default:
		return ""
	}
}

func (s LiverStatus) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *LiverStatus) UnmarshalText(b []byte) error {
	switch v := string(b); v {
	case valueAnnounced:
		*s = LiverStatusAnnounced
	case valueDebuted:
		*s = LiverStatusDebuted
	case valueRetired:
		*s = LiverStatusRetired
	default:
		return fmt.Errorf("unknown status: %q", v)
	}
	return nil
}

func (s LiverStatus) MarshalGQLContext(_ context.Context, w io.Writer) error {
	fmt.Fprint(w, strconv.Quote(s.String()))
	return nil
}

func (s *LiverStatus) UnmarshalGQLContext(_ context.Context, v any) error {
	sv, ok := v.(string)
	if !ok {
		return errors.New("LiverStatus must be a string")
	}
	return s.UnmarshalText([]byte(sv))
}

type GroupCursor struct {
	GroupID uint64
}

func (c *GroupCursor) IsEmpty() bool {
	return c == nil || c == &GroupCursor{}
}

func (c *GroupCursor) Encode() string {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return cursorEncoding.EncodeToString(b)
}

type Group struct {
	Name string `json:"name" db:"name"`
	ID   uint64 `db:"liver_group_id"`
}

type LiverGroupEdge struct {
	Node *domain.Group `json:"node"`
}

var _ Edge = (*LiverGroupEdge)(nil)

func (g *LiverGroupEdge) Cursor() string {
	if g == nil || g.Node == nil {
		return ""
	}
	return (&GroupCursor{GroupID: g.Node.ID}).Encode()
}
