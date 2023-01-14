package models

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
)

func ParsePermissionClaim(v any, ok bool) *Permission {
	if !ok {
		return nil
	}
	xs, ok := v.([]any)
	if !ok {
		return nil
	}
	ss := make([]Scope, 0, len(xs))
	for _, x := range xs {
		if s, ok := x.(string); ok {
			if p, err := ParseScope(s); err == nil {
				ss = append(ss, p)
			}
		}
	}
	return NewPermission(ss...)
}

var exists = struct{}{}

type Permission struct {
	sync.Mutex
	m map[Scope]struct{}
}

func NewPermission(scopes ...Scope) *Permission {
	ps := &Permission{m: make(map[Scope]struct{}, len(scopes))}
	for _, scope := range scopes {
		ps.m[scope] = exists
	}
	return ps
}

func (ps *Permission) Strings() []string {
	xs := make([]string, 0, len(ps.m))
	for p := range ps.m {
		xs = append(xs, p.String())
	}
	sort.Strings(xs)
	return xs
}

func (ps *Permission) Add(scopes ...Scope) {
	ps.Lock()
	defer ps.Unlock()
	for _, scope := range scopes {
		ps.m[scope] = exists
	}
}

func (ps *Permission) IsSuperSetOf(other *Permission) bool {
	ps.Lock()
	defer ps.Unlock()
	other.Lock()
	defer other.Unlock()
	if len(ps.m) < len(other.m) {
		return false
	}
	for req := range other.m {
		if _, ok := ps.m[req]; !ok {
			return false
		}
	}
	return true
}

var (
	ErrInvalidScope = errors.New("invalid scope")
	InvalidScope    = Scope("")
)

func ParseScope(v string) (Scope, error) {
	switch v {
	case ScopeWrite.String():
		return ScopeWrite, nil
	case ScopeRead.String():
		return ScopeRead, nil
	default:
		return InvalidScope, ErrInvalidScope
	}
}

type Scope string

const (
	ScopeRead  Scope = "read"
	ScopeWrite Scope = "write"
)

var AllScope = []Scope{
	ScopeWrite,
}

func (e Scope) IsValid() bool {
	switch e {
	case ScopeWrite, ScopeRead:
		return true
	}
	return false
}

func (e Scope) String() string {
	return string(e)
}

func (e *Scope) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = Scope(strings.ToLower(str))
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid Scope", str)
	}
	return nil
}

func (e Scope) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
