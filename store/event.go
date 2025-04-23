package store

import "time"

type EventCount struct {
	Total   int
	Pending int
}

type EventStore interface {
	AddPending(eventName, id string) (EventCount, error)
	ContainsEvent(eventName, id string) bool
	GetCount(eventName string) (EventCount, error)
	Fulfill(eventName string, number int) (EventCount, error)
}

type EventStoreError string

const NoChange = EventStoreError("event-store: no change")

func (e EventStoreError) Error() string { return string(e) }

const eventLifetime time.Duration = 24 * time.Hour
