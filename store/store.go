package store

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis"
)

const sessionLifetime time.Duration = 24 * 60 * time.Hour

type Session struct {
	Access  string `json:"access"`
	Refresh string `json:"refresh"`
	UserId  string `json:"user_id"`
}

type SessionStore interface {
	SetSession(token string, session Session) error
	GetSession(token string) (*Session, error)
	DeleteSession(token string) error
}

type EventCount struct {
	Total   int
	Pending int
}

type EventStore interface {
	AddPending(eventName string) (EventCount, error)
	GetCount(eventName string) (EventCount, error)
	Fulfill(eventName string, number int) (EventCount, error)
}

type EventStoreError string

const NoChange = EventStoreError("event-store: no change")

func (e EventStoreError) Error() string { return string(e) }

type inMemorySessionStore struct {
	mutex    sync.RWMutex
	sessions map[string]Session
}

func NewInMemorySessionStore() SessionStore {
	return &inMemorySessionStore{
		sessions: make(map[string]Session),
	}
}

func (s *inMemorySessionStore) SetSession(token string, session Session) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	key := "session:" + token
	s.sessions[key] = session
	return nil
}

func (s *inMemorySessionStore) GetSession(token string) (*Session, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	key := "session:" + token
	session, ok := s.sessions[key]
	if !ok {
		return nil, nil
	}
	return &session, nil
}

func (s *inMemorySessionStore) DeleteSession(token string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	key := "session:" + token
	delete(s.sessions, key)
	return nil
}

type inMemoryEventStore struct {
	mutex  sync.RWMutex
	events map[string]EventCount
}

func NewInMemoryEventStore() EventStore {
	return &inMemoryEventStore{
		events: make(map[string]EventCount),
	}
}

func (s *inMemoryEventStore) AddPending(eventName string) (EventCount, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	count := s.events[eventName]
	count.Pending++
	count.Total++
	s.events[eventName] = count
	return count, nil
}

func (s *inMemoryEventStore) GetCount(eventName string) (EventCount, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	count := s.events[eventName]
	return count, nil
}

func (s *inMemoryEventStore) Fulfill(eventName string, number int) (EventCount, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	count := s.events[eventName]

	var numFulfilled int
	if number == -1 {
		numFulfilled = count.Pending
	} else {
		numFulfilled = max(min(count.Pending, number), 0)
	}

	if numFulfilled == 0 {
		return EventCount{}, NoChange
	}

	count.Pending -= numFulfilled
	s.events[eventName] = count
	return count, nil
}

type redisSessionStore struct {
	client *redis.Client
}

type redisStore struct {
	client *redis.Client
}

type MultiStore interface {
	SessionStore
	EventStore
}

func NewRedisStore() MultiStore {
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	return &redisStore{
		client: rdb,
	}
}

func (s redisStore) GetSession(token string) (*Session, error) {
	key := "session:" + token
	data, err := s.client.Get(key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

func (s redisStore) SetSession(token string, session Session) error {
	bytes, err := json.Marshal(session)
	if err != nil {
		return err
	}

	key := "session:" + token
	return s.client.Set(key, bytes, sessionLifetime).Err()
}

func (s redisStore) DeleteSession(token string) error {
	key := "session:" + token
	return s.client.Del(key).Err()
}

func (s *redisStore) AddPending(eventName string) (EventCount, error) {
	pendingKey := eventName + ":pending"
	totalKey := eventName + ":total"
	var pendingCmd, totalCmd *redis.IntCmd

	err := s.client.Watch(func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(func(pipe redis.Pipeliner) error {
			pendingCmd = pipe.Incr(pendingKey)
			totalCmd = pipe.Incr(totalKey)
			return nil
		})
		return err
	}, pendingKey, totalKey)

	if err != nil {
		return EventCount{}, err
	}

	count := EventCount{
		Pending: int(pendingCmd.Val()),
		Total:   int(totalCmd.Val()),
	}
	return count, nil
}

func (s *redisStore) Fulfill(eventName string, number int) (EventCount, error) {
	pendingKey := eventName + ":pending"
	pending, err := s.client.Get(pendingKey).Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	totalKey := eventName + ":total"
	total, err := s.client.Get(totalKey).Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	var numFulfilled int
	if number == -1 {
		numFulfilled = pending
	} else {
		numFulfilled = max(min(pending, number), 0)
	}

	if numFulfilled == 0 {
		return EventCount{}, NoChange
	}

	val, err := s.client.DecrBy(pendingKey, int64(numFulfilled)).Result()
	if err != nil {
		return EventCount{}, err
	}

	count := EventCount{
		Pending: int(val),
		Total:   total,
	}
	return count, nil
}

func (s *redisStore) GetCount(eventName string) (EventCount, error) {
	pending, err := s.client.Get(eventName + ":pending").Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	total, err := s.client.Get(eventName + ":total").Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	count := EventCount{
		Pending: pending,
		Total:   total,
	}
	return count, nil
}
