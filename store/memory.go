package store

import (
	"sync"
)

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

func (s *inMemorySessionStore) ContainsSession(token string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	key := "session:" + token
	_, ok := s.sessions[key]
	return ok
}

type inMemoryEventStore struct {
	mutex  sync.RWMutex
	events map[string]EventCount
	ids    map[string]bool
}

func NewInMemoryEventStore() EventStore {
	return &inMemoryEventStore{
		events: make(map[string]EventCount),
		ids:    make(map[string]bool),
	}
}

func (s *inMemoryEventStore) AddPending(eventName, id string) (EventCount, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	count := s.events[eventName]
	count.Pending++
	count.Total++
	s.events[eventName] = count
	s.ids[eventName+":id:"+id] = true
	return count, nil
}

func (s *inMemoryEventStore) ContainsEvent(eventName, id string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	_, ok := s.ids[eventName+":id:"+id]
	return ok
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
