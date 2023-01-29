package models_test

import (
	"testing"

	"github.com/aereal/enjoy-opentelemetry/graph/models"
)

func TestGroupCursor_IsBefore(t *testing.T) {
	testCases := []struct {
		name string
		lhs  *models.GroupCursor
		rhs  *models.GroupCursor
		want bool
	}{
		{
			name: "nil vs nil",
			want: true,
		},
		{
			name: "nil vs 1",
			rhs:  &models.GroupCursor{GroupID: 1},
			want: true,
		},
		{
			name: "1 vs 1",
			lhs:  &models.GroupCursor{GroupID: 1},
			rhs:  &models.GroupCursor{GroupID: 1},
			want: false,
		},
		{
			name: "1 vs 2",
			lhs:  &models.GroupCursor{GroupID: 1},
			rhs:  &models.GroupCursor{GroupID: 2},
			want: false,
		},
		{
			name: "2 vs 1",
			lhs:  &models.GroupCursor{GroupID: 2},
			rhs:  &models.GroupCursor{GroupID: 1},
			want: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.lhs.IsBefore(tc.rhs)
			if got != tc.want {
				t.Errorf("want=%v got=%v", tc.want, got)
			}
		})
	}
}
