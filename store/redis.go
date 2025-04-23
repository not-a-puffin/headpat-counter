package store

import (
	"github.com/go-redis/redis"
)

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
	bytes, err := s.client.Get(key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	session, err := decodeSession(string(bytes))

	return &session, nil
}

func (s redisStore) SetSession(token string, session Session) error {
	bytes, err := encodeSession(session)
	if err != nil {
		return err
	}

	key := "session:" + token
	return s.client.Set(key, bytes, sessionLifetime).Err()
}

func (s redisStore) DeleteSession(token string) error {
	key := "session:" + token
	return s.client.Unlink(key).Err()
}

func (s redisStore) ContainsSession(token string) bool {
	key := "session:" + token
	count := s.client.Exists(key).Val()
	return count > 0
}

func (s *redisStore) AddPending(eventName, id string) (EventCount, error) {
	pendingKey := "event:" + eventName + ":pending"
	totalKey := "event:" + eventName + ":total"
	idKey := "event:" + eventName + ":id:" + id
	var pendingCmd, totalCmd *redis.IntCmd

	err := s.client.Watch(func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(func(pipe redis.Pipeliner) error {
			pendingCmd = pipe.Incr(pendingKey)
			totalCmd = pipe.Incr(totalKey)
			pipe.Set(idKey, "", eventLifetime)
			return nil
		})
		return err
	}, pendingKey, totalKey, idKey)

	if err != nil {
		return EventCount{}, err
	}

	count := EventCount{
		Pending: int(pendingCmd.Val()),
		Total:   int(totalCmd.Val()),
	}
	return count, nil
}

func (s *redisStore) ContainsEvent(eventName, id string) bool {
	idKey := "event:" + eventName + ":id:" + id
	count := s.client.Exists(idKey).Val()
	return count > 0
}

func (s *redisStore) Fulfill(eventName string, number int) (EventCount, error) {
	pendingKey := "event:" + eventName + ":pending"
	pending, err := s.client.Get(pendingKey).Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	totalKey := "event:" + eventName + ":total"
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
	pendingKey := "event:" + eventName + ":pending"
	pending, err := s.client.Get(pendingKey).Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	totalKey := "event:" + eventName + ":total"
	total, err := s.client.Get(totalKey).Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	count := EventCount{
		Pending: pending,
		Total:   total,
	}
	return count, nil
}
