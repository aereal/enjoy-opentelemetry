package domain

type Group struct {
	Name string `json:"name" db:"name"`
	ID   uint64 `db:"liver_group_id"`
}

type LiverBelongingGroup struct {
	Group
	LiverID uint64 `db:"liver_id"`
}
