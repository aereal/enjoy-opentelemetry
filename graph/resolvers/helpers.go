package resolvers

import (
	"fmt"

	"github.com/aereal/enjoy-opentelemetry/graph/models"
	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	"github.com/doug-martin/goqu/v9/exp"
)

var (
	dialect     = goqu.Dialect("mysql")
	liversTable = dialect.From("livers")
)

func toOrderBy(o *models.LiverOrder) exp.OrderedExpression {
	var column exp.IdentifierExpression
	switch f := o.Field; f {
	case models.LiverOrderFieldDatabaseID:
		column = goqu.C("liver_id")
	default:
		panic(fmt.Errorf("[BUG] unknown field: %s", f))
	}
	switch d := o.Direction; d {
	case models.OrderDirectionAsc:
		return column.Asc()
	case models.OrderDirectionDesc:
		return column.Desc()
	default:
		panic(fmt.Errorf("[BUG] unknown direction: %s", d))
	}
}
