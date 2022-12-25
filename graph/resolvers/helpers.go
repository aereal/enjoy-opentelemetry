package resolvers

import (
	"fmt"
	"strings"

	"github.com/aereal/enjoy-opentelemetry/graph/models"
)

func toOrderBy(o *models.LiverOrder) string {
	var (
		column string
		dir    string
	)
	switch o.Field {
	case models.LiverOrderFieldDatabaseID:
		column = "liver_id"
	default:
		panic(fmt.Errorf("[BUG] unknown field: %s", o.Field))
	}
	switch o.Direction {
	case models.OrderDirectionAsc, models.OrderDirectionDesc:
		dir = strings.ToLower(o.Direction.String())
	default:
		panic(fmt.Errorf("[BUG] unknown direction: %s", o.Direction))
	}
	return fmt.Sprintf("order by %s %s", column, dir)
}
