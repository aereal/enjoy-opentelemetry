package models_test

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"github.com/aereal/enjoy-opentelemetry/graph/models"
	"github.com/google/go-cmp/cmp"
)

var cursorTestCases = []struct {
	name         string
	cursor       *models.Cursor
	encodedValue string
}{
	{
		name:         "int",
		cursor:       &models.Cursor{Type: "Group", Value: []byte("1234")},
		encodedValue: "eyJUeXBlIjoiR3JvdXAiLCJWYWx1ZSI6MTIzNH0=",
	},
	{
		name:         "string",
		cursor:       &models.Cursor{Type: "Group", Value: []byte(`"abc"`)},
		encodedValue: "eyJUeXBlIjoiR3JvdXAiLCJWYWx1ZSI6ImFiYyJ9",
	},
}

func TestCursor_Encode(t *testing.T) {
	for _, tc := range cursorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.cursor.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.encodedValue {
				t.Errorf("want=%q got=%q", tc.encodedValue, got)
			}
		})
	}
}

func TestCursor_MarshalGQLContext(t *testing.T) {
	for _, tc := range cursorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			if err := tc.cursor.MarshalGQLContext(context.Background(), buf); err != nil {
				t.Fatal(err)
			}
			got := buf.String()
			if got != strconv.Quote(tc.encodedValue) {
				t.Errorf("want=%q got=%q", tc.encodedValue, got)
			}
		})
	}
}

func TestCursor_UnmarshalGQLContext(t *testing.T) {
	for _, tc := range cursorTestCases {
		t.Run(tc.name, func(t *testing.T) {
			got := &models.Cursor{}
			if err := got.UnmarshalGQLContext(context.Background(), tc.encodedValue); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.cursor, got); diff != "" {
				t.Errorf("-want, +got:\n%s", diff)
			}
		})
	}
}
