package store

import (
	"sync"

	"github.com/go-redis/redis"
)

type Session struct {
	Access  string
	Refresh string
}

type SessionStore interface {
	SetSession(token string, session Session) error
	GetSession(token string) (*Session, error)
	DeleteSession(token string) error
}

type Count struct {
	Total       int
	Unfulfilled int
}

type CounterStore interface {
	LoadCount() (Count, error)
	SaveCount(value Count) error
}

type Store interface {
	CounterStore
	SessionStore
}

type memoryStore struct {
	mutex            sync.RWMutex
	sessions         map[string]Session
	totalCount       int
	unfulfilledCount int
}

func (s *memoryStore) SetSession(token string, session Session) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.sessions[token] = session
	return nil
}

func (s *memoryStore) GetSession(token string) (*Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	session, ok := s.sessions[token]
	if !ok {
		return nil, nil
	}
	return &session, nil
}

func (s *memoryStore) DeleteSession(token string) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	delete(s.sessions, token)
	return nil
}

func (s *memoryStore) LoadCount() (Count, error) {
	count := Count{
		Total:       s.totalCount,
		Unfulfilled: s.unfulfilledCount,
	}
	return count, nil
}

func (s *memoryStore) SaveCount(count Count) error {
	s.totalCount = count.Total
	s.unfulfilledCount = count.Unfulfilled
	return nil
}

func NewMemoryStore() Store {
	return &memoryStore{
		sessions: make(map[string]Session),
	}
}

type redisStore struct {
	client *redis.Client
}

// DeleteSession implements Store.
func (r redisStore) DeleteSession(token string) error {
	panic("unimplemented")
}

// GetSession implements Store.
func (r redisStore) GetSession(token string) (*Session, error) {
	panic("unimplemented")
}

// LoadCount implements Store.
func (r redisStore) LoadCount() (Count, error) {
	panic("unimplemented")
}

// SaveCount implements Store.
func (r redisStore) SaveCount(value Count) error {
	panic("unimplemented")
}

// SetSession implements Store.
func (r redisStore) SetSession(token string, session Session) error {
	panic("unimplemented")
}

func NewRedisStore(addr string) Store {
	return redisStore{}
}
