package store

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type redisStore struct {
	client *redis.Client
}

type MultiStore interface {
	EventStore
	SessionStore
	ScoreboardStore
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
	ctx := context.Background()
	key := "session:" + token
	bytes, err := s.client.Get(ctx, key).Bytes()
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

	ctx := context.Background()
	key := "session:" + token
	return s.client.Set(ctx, key, bytes, sessionLifetime).Err()
}

func (s redisStore) DeleteSession(token string) error {
	ctx := context.Background()
	key := "session:" + token
	return s.client.Unlink(ctx, key).Err()
}

func (s redisStore) ContainsSession(token string) bool {
	ctx := context.Background()
	key := "session:" + token
	count := s.client.Exists(ctx, key).Val()
	return count > 0
}

func (s *redisStore) AddPendingEvent(eventName, id string) (EventCount, error) {
	ctx := context.Background()

	pendingKey := "event:" + eventName + ":pending"
	totalKey := "event:" + eventName + ":total"
	idKey := "event:" + eventName + ":id:" + id
	var pendingCmd, totalCmd *redis.IntCmd

	err := s.client.Watch(ctx, func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pendingCmd = pipe.Incr(ctx, pendingKey)
			totalCmd = pipe.Incr(ctx, totalKey)
			pipe.Set(ctx, idKey, "", eventLifetime)
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

func (s *redisStore) EventExists(eventName, id string) bool {
	ctx := context.Background()
	key := "event:" + eventName + ":id:" + id
	count := s.client.Exists(ctx, key).Val()
	return count > 0
}

func (s *redisStore) FulfillEvent(eventName string, number int) (EventCount, error) {
	ctx := context.Background()

	pendingKey := "event:" + eventName + ":pending"
	pending, err := s.client.Get(ctx, pendingKey).Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	totalKey := "event:" + eventName + ":total"
	total, err := s.client.Get(ctx, totalKey).Int()
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

	val, err := s.client.DecrBy(ctx, pendingKey, int64(numFulfilled)).Result()
	if err != nil {
		return EventCount{}, err
	}

	count := EventCount{
		Pending: int(val),
		Total:   total,
	}
	return count, nil
}

func (s *redisStore) EventCount(eventName string) (EventCount, error) {
	ctx := context.Background()

	pendingKey := "event:" + eventName + ":pending"
	pending, err := s.client.Get(ctx, pendingKey).Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	totalKey := "event:" + eventName + ":total"
	total, err := s.client.Get(ctx, totalKey).Int()
	if err != nil && err != redis.Nil {
		return EventCount{}, err
	}

	count := EventCount{
		Pending: pending,
		Total:   total,
	}
	return count, nil
}

func (s *redisStore) ScoreboardIncr(boardName, userName string, points float64) error {
	ctx := context.Background()
	key := "scoreboard:" + boardName
	return s.client.ZIncrBy(ctx, key, points, userName).Err()
}

func (s *redisStore) GetLeaderboard(boardName string, count int) ([]ScoreEntry, error) {
	if count <= 0 {
		return nil, fmt.Errorf("Leaderboard count must be a positive number")
	}
	if count > 100 {
		return nil, fmt.Errorf("Leaderboard count must not be greater than 100")
	}

	ctx := context.Background()
	key := "scoreboard:" + boardName
	results, err := s.client.ZRevRangeWithScores(ctx, key, 0, int64(count-1)).Result()
	if err != nil && err != redis.Nil {
		return nil, err
	}

	scores := make([]ScoreEntry, len(results))
	for i, value := range results {
		scores[i] = ScoreEntry{Rank: int64(i + 1), Score: value.Score, User: value.Member.(string)}
	}
	return scores, nil
}

func (s *redisStore) GetScoreByUser(boardName string, userName string) (ScoreEntry, error) {
	ctx := context.Background()
	key := "scoreboard:" + boardName
	result, err := s.client.ZRevRankWithScore(ctx, key, userName).Result()
	if err != nil && err != redis.Nil {
		return ScoreEntry{}, err
	}

	// Rank -1 indicates no record
	if err == redis.Nil {
		return ScoreEntry{Rank: -1, User: userName}, nil
	}

	score := ScoreEntry{Rank: result.Rank + 1, Score: result.Score, User: userName}
	return score, nil
}

func (s *redisStore) ResetScoreboard(boardName string) error {
	ctx := context.Background()
	key := "scoreboard:" + boardName
	return s.client.Unlink(ctx, key).Err()
}
