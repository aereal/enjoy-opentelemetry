package observability

import (
	"go.opentelemetry.io/otel/attribute"
)

var (
	keyDBTable = attribute.Key("db.table")

	MetricNames = struct {
		RepositoryFetchedResultCount, RepositoryInsertedCount string
	}{
		RepositoryFetchedResultCount: "domain.repo.fetched_result_count",
		RepositoryInsertedCount:      "domain.repo.inserted_count",
	}
)

func AttrDBTable(table string) attribute.KeyValue { return keyDBTable.String(table) }
