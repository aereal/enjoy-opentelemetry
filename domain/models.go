package domain

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/99designs/gqlgen/graphql"
)

type Group struct {
	Name string `json:"name" db:"name"`
	ID   uint64 `db:"liver_group_id"`
}

type LiverBelongingGroup struct {
	Group
	LiverID uint64 `db:"liver_id"`
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
