package store

import "time"

const streamLifetime time.Duration = 365 * 24 * time.Hour

type StreamStore interface {
	AddStreamStartEvent(streamId string, timestamp time.Time) error
	AddOutOfStockEvent(streamId string, timestamp time.Time) error
	IsOutOfStock() bool
}
