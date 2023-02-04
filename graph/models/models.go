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

	"github.com/99designs/gqlgen/graphql"
	"github.com/aereal/enjoy-opentelemetry/domain"
)

type Edge interface {
	Cursor() (*Cursor, error)
}

func NewPageInfo[E Edge](edges []E, hasNext bool) (*PageInfo, error) {
	if len(edges) == 0 {
		return &PageInfo{}, nil
	}
	lastIdx := len(edges) - 1
	pi := &PageInfo{HasNextPage: hasNext}
	{
		cursor, err := edges[0].Cursor()
		if err != nil {
			return nil, err
		}
		pi.StartCursor = cursor
	}
	{
		cursor, err := edges[lastIdx].Cursor()
		if err != nil {
			return nil, err
		}
		pi.EndCursor = cursor
	}
	return pi, nil
}

type LiverEdge struct {
	*domain.Liver
}

var _ Edge = &LiverEdge{}

func (e *LiverEdge) Cursor() (*Cursor, error) {
	cursor := &Cursor{Type: "LiverEdge"}
	var err error
	cursor.Value, err = json.Marshal(&LiverCursorValue{LiverID: e.ID})
	if err != nil {
		return nil, err
	}
	return cursor, nil
}

var (
	cursorEncoding = base64.StdEncoding
)

type LiverCursorValue struct {
	LiverID uint64
}

type GroupCursorValue struct {
	GroupID uint64
}

type LiverGroupEdge struct {
	Node *domain.Group `json:"node"`
}

var _ Edge = (*LiverGroupEdge)(nil)

func (g *LiverGroupEdge) Cursor() (*Cursor, error) {
	cursor := &Cursor{Type: "LiverGroupEdge"}
	var err error
	cursor.Value, err = json.Marshal(&GroupCursorValue{GroupID: g.Node.ID})
	if err != nil {
		return nil, err
	}
	return cursor, nil
}

type LiverGroupConnetion struct {
	Edges   []*LiverGroupEdge `json:"edges"`
	HasNext bool
}

type Cursor struct {
	Type  string
	Value json.RawMessage
}

var _ interface {
	graphql.ContextMarshaler
	graphql.ContextUnmarshaler
	encoding.TextMarshaler
	encoding.TextUnmarshaler
} = (*Cursor)(nil)

func ParseCursor[T any](after *string, v *T) error {
	if after == nil {
		return nil
	}
	cursor := &Cursor{}
	if err := cursor.UnmarshalText([]byte(*after)); err != nil {
		return fmt.Errorf("Cursor.UnmarshalText: %w", err)
	}
	if err := json.Unmarshal(cursor.Value, v); err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}
	return nil
}

func (c *Cursor) Encode() (string, error) {
	b, err := c.MarshalText()
	if err != nil {
		return "", fmt.Errorf("Cursor.Encode: %w", err)
	}
	return string(b), nil
}

type underlyingCursor struct {
	Type  string
	Value json.RawMessage
}

func (c *Cursor) MarshalText() ([]byte, error) {
	if c == nil {
		return nil, errors.New("Cursor is nil")
	}
	b, err := json.Marshal(underlyingCursor{Type: c.Type, Value: c.Value})
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}
	dst := make([]byte, cursorEncoding.EncodedLen(len(b)))
	cursorEncoding.Encode(dst, b)
	return dst, nil
}

func (c *Cursor) UnmarshalText(v []byte) error {
	decoded, err := cursorEncoding.DecodeString(string(v))
	if err != nil {
		return fmt.Errorf("DecodeString: %w", err)
	}
	var uc underlyingCursor
	if err := json.Unmarshal(decoded, &uc); err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}
	c.Type = uc.Type
	c.Value = uc.Value
	return nil
}

func (c Cursor) MarshalGQLContext(_ context.Context, w io.Writer) error {
	s, err := c.Encode()
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, strconv.Quote(s)); err != nil {
		return err
	}
	return nil
}

func (c *Cursor) UnmarshalGQLContext(_ context.Context, v any) error {
	switch v := v.(type) {
	case string:
		if err := c.UnmarshalText([]byte(v)); err != nil {
			return err
		}
	case []byte:
		if err := c.UnmarshalText(v); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}
