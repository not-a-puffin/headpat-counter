package store

import "time"

const streamLifetime time.Duration = 365 * 24 * time.Hour

type StreamStore interface {
	AddStreamStartEvent(id, startTime string) error
	AddNumRedeemedThisStream(streamId string, count int) error
	GetNumRedeemedThisStream(streamId string) (int, error)
	AddOutOfStockEvent(streamId, timestamp string) error
}
