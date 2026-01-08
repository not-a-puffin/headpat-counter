package store

import "time"

type EventCount struct {
	Total   int
	Pending int
}

type EventStore interface {
	AddPendingEvent(eventName, id string) (EventCount, error)
	EventExists(eventName, id string) bool
	EventCount(eventName string) (EventCount, error)
	FulfillEvent(eventName string, number int) (EventCount, error)
	AddStreamStartEvent(id, startTime string) error
	AddNumRedeemedThisStream(streamId string, count int) error
	GetNumRedeemedThisStream(streamId string) (int, error)
	AddOutOfStockEvent(streamId, timestamp string) error
}

type EventStoreError string

const NoChange = EventStoreError("event-store: no change")

func (e EventStoreError) Error() string { return string(e) }

const eventLifetime time.Duration = 24 * time.Hour
const streamLifetime time.Duration = 365 * 24 * time.Hour
