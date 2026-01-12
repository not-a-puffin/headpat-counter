package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Store interface {
	HeadpatStore
	ScoreboardStore
	SessionStore
	StreamStore
	TokenStore
}

type redisStore struct {
	client *redis.Client
}

const (
	redisHostAddr string = "localhost:6379"
)

func NewRedisStore() Store {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisHostAddr,
	})
	return &redisStore{
		client: rdb,
	}
}

func (st redisStore) GetSession(token string) (*Session, error) {
	ctx := context.Background()
	key := "session:" + token
	bytes, err := st.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var session Session
	if err := json.Unmarshal(bytes, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (st redisStore) SetSession(token string, session Session) error {
	bytes, err := json.Marshal(session)
	if err != nil {
		return err
	}

	ctx := context.Background()
	key := "session:" + token
	return st.client.Set(ctx, key, bytes, 0).Err()
}

func (st redisStore) DeleteSession(token string) error {
	ctx := context.Background()
	key := "session:" + token
	return st.client.Unlink(ctx, key).Err()
}

func (st redisStore) ContainsSession(token string) bool {
	ctx := context.Background()
	key := "session:" + token
	count := st.client.Exists(ctx, key).Val()
	return count > 0
}

func (st redisStore) GetTokenPair(id string) (*TokenPair, error) {
	ctx := context.Background()
	key := "token:" + id
	bytes, err := st.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	tokenPair, err := decodeTokenPair(string(bytes))
	if err != nil {
		return nil, err
	}

	return &tokenPair, nil
}

func (st redisStore) SetTokenPair(id string, tokenPair TokenPair) error {
	bytes, err := encodeTokenPair(tokenPair)
	if err != nil {
		return err
	}

	ctx := context.Background()
	key := "token:" + id

	// Token does not expire
	return st.client.Set(ctx, key, bytes, 0).Err()
}

func (s *redisStore) AddPendingEvent(eventName, id string) (HeadpatCount, error) {
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
		return HeadpatCount{}, err
	}

	count := HeadpatCount{
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

func (s *redisStore) FulfillEvent(eventName string, number int) (HeadpatCount, error) {
	ctx := context.Background()

	pendingKey := "event:" + eventName + ":pending"
	pending, err := s.client.Get(ctx, pendingKey).Int()
	if err != nil && err != redis.Nil {
		return HeadpatCount{}, err
	}

	totalKey := "event:" + eventName + ":total"
	total, err := s.client.Get(ctx, totalKey).Int()
	if err != nil && err != redis.Nil {
		return HeadpatCount{}, err
	}

	var numFulfilled int
	if number == -1 {
		numFulfilled = pending
	} else {
		numFulfilled = max(min(pending, number), 0)
	}

	if numFulfilled == 0 {
		return HeadpatCount{}, NoChange
	}

	val, err := s.client.DecrBy(ctx, pendingKey, int64(numFulfilled)).Result()
	if err != nil {
		return HeadpatCount{}, err
	}

	count := HeadpatCount{
		Pending: int(val),
		Total:   total,
	}
	return count, nil
}

func (s *redisStore) GetHeadpatCount(eventName string) (HeadpatCount, error) {
	ctx := context.Background()

	pendingKey := "event:" + eventName + ":pending"
	pending, err := s.client.Get(ctx, pendingKey).Int()
	if err != nil && err != redis.Nil {
		return HeadpatCount{}, err
	}

	totalKey := "event:" + eventName + ":total"
	total, err := s.client.Get(ctx, totalKey).Int()
	if err != nil && err != redis.Nil {
		return HeadpatCount{}, err
	}

	count := HeadpatCount{
		Pending: pending,
		Total:   total,
	}
	return count, nil
}

func (s *redisStore) AddStreamStartEvent(id, startTime string) error {
	ctx := context.Background()
	key := "stream:" + id + ":start"
	return s.client.Set(ctx, key, startTime, streamLifetime).Err()
}

func (s *redisStore) AddNumRedeemedThisStream(streamId string, count int) error {
	ctx := context.Background()
	key := "stream:" + streamId + ":num-redeemed"
	return s.client.Set(ctx, key, count, streamLifetime).Err()
}

func (s *redisStore) GetNumRedeemedThisStream(streamId string) (int, error) {
	ctx := context.Background()
	key := "stream:" + streamId + ":num-redeemed"
	count, err := s.client.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		return 0, err
	}
	return count, nil
}

func (s *redisStore) AddOutOfStockEvent(id, timestamp string) error {
	ctx := context.Background()
	key := "stream:" + id + ":out-of-stock"
	return s.client.Set(ctx, key, timestamp, streamLifetime).Err()
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
	rank, err := s.client.ZRevRank(ctx, key, userName).Result()
	if err != nil && err != redis.Nil {
		return ScoreEntry{}, err
	}

	score, err := s.client.ZScore(ctx, key, userName).Result()
	if err != nil && err != redis.Nil {
		return ScoreEntry{}, err
	}

	// Rank -1 indicates no record
	if err == redis.Nil {
		return ScoreEntry{Rank: -1, User: userName}, nil
	}

	entry := ScoreEntry{Rank: rank + 1, Score: score, User: userName}
	return entry, nil
}

func (s *redisStore) ResetScoreboard(boardName string) error {
	ctx := context.Background()
	key := "scoreboard:" + boardName
	return s.client.Unlink(ctx, key).Err()
}
