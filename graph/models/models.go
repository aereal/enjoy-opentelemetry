package models

type Liver struct {
	ID   uint64 `db:"liver_id"`
	Name string `json:"name" db:"name"`
	Age  *int   `json:"age" db:"age"`
}
