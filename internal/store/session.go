package store

import (
	"time"
)

type Session struct {
	UserId  string    `json:"user_id"`
	Expires time.Time `json:"expires"`
}

type SessionStore interface {
	SetSession(token string, session Session) error
	GetSession(token string) (*Session, error)
	DeleteSession(token string) error
	ContainsSession(token string) bool
}
