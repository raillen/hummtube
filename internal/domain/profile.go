package domain

import "time"

type ProfileKind string

const (
	ProfileKindPersistent ProfileKind = "persistent"
	ProfileKindGuest      ProfileKind = "guest"
)

type Profile struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Kind      ProfileKind `json:"kind"`
	CreatedAt time.Time   `json:"created_at"`
}

func (p Profile) IsGuest() bool { return p.Kind == ProfileKindGuest }
