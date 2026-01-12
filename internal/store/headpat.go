package store

import "time"

type HeadpatCount struct {
	Total   int
	Pending int
}

type HeadpatStore interface {
	AddPendingEvent(eventName, id string) (HeadpatCount, error)
	EventExists(eventName, id string) bool
	GetHeadpatCount(eventName string) (HeadpatCount, error)
	FulfillEvent(eventName string, number int) (HeadpatCount, error)
}

type EventStoreError string

const NoChange = EventStoreError("event-store: no change")

func (e EventStoreError) Error() string { return string(e) }

const eventLifetime time.Duration = 24 * time.Hour
