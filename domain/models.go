package domain

type Group struct {
	Name string `json:"name" db:"name"`
	ID   uint64 `db:"liver_group_id"`
}
