package store

import "time"

type HeadpatCount struct {
	Total   int
	Pending int
}

type HeadpatStore interface {
	AddPendingHeadpat(id string) (HeadpatCount, error)
	AddRemainingHeadpats(streamId string, numRedeemed int) error
	HeadpatExists(id string) bool
	GetHeadpatCount() (HeadpatCount, error)
	FulfillHeadpats(number int) (HeadpatCount, error)
}

type EventStoreError string

const NoChange = EventStoreError("event-store: no change")

func (e EventStoreError) Error() string { return string(e) }

const eventLifetime time.Duration = 12 * time.Hour
