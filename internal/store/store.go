package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

func (s *redisStore) AddPendingHeadpat(id string) (HeadpatCount, error) {
	ctx := context.Background()

	pendingKey := "event:headpat:pending"
	totalKey := "event:headpat:total"
	idKey := "event:headpat:id:" + id
	setKey := "stream:headpats"
	ts := time.Now().Unix()
	entry := redis.Z{Member: id, Score: float64(ts)}
	var pendingCmd, totalCmd *redis.IntCmd

	err := s.client.Watch(ctx, func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pendingCmd = pipe.Incr(ctx, pendingKey)
			totalCmd = pipe.Incr(ctx, totalKey)
			pipe.Set(ctx, idKey, "", eventLifetime)
			pipe.ZAdd(ctx, setKey, entry)
			return nil
		})
		return err
	}, pendingKey, totalKey, setKey)

	if err != nil {
		return HeadpatCount{}, err
	}

	count := HeadpatCount{
		Pending: int(pendingCmd.Val()),
		Total:   int(totalCmd.Val()),
	}
	return count, nil
}

func (s *redisStore) AddRemainingHeadpats(streamId string, count int) error {
	ctx := context.Background()

	// Get ts of last stream
	streamOnlineResult := s.client.ZPopMax(ctx, "stream:online", 1).Val()
	if len(streamOnlineResult) == 0 {
		err := fmt.Errorf("No stream found")
		return err
	}

	// Make sure stream matches expected
	streamStart := streamOnlineResult[0]
	if streamStart.Member != streamId {
		err := fmt.Errorf("Stream ID does not match")
		return err
	}

	// Get headpats from this stream
	ts := time.Unix(int64(streamStart.Score), 0).Add(-5 * time.Minute)
	min := fmt.Sprintf("(%d", ts.Unix())
	countThisStream := int(s.client.ZCount(ctx, "stream:headpats", min, "+inf").Val())
	if countThisStream == 0 {
		err := fmt.Errorf("No headpats found this stream")
		return err
	}

	diff := count - max(count, countThisStream)
	if diff == 0 {
		return NoChange
	}

	pendingKey := "event:headpat:pending"
	totalKey := "event:headpat:total"
	err := s.client.Watch(ctx, func(tx *redis.Tx) error {
		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Incr(ctx, pendingKey)
			pipe.Incr(ctx, totalKey)
			return nil
		})
		return err
	}, pendingKey, totalKey)

	if err != nil {
		return err
	}

	return nil
}

func (s *redisStore) HeadpatExists(id string) bool {
	ctx := context.Background()
	key := "event:headpat:id:" + id
	count := s.client.Exists(ctx, key).Val()
	return count > 0
}

func (s *redisStore) FulfillHeadpats(number int) (HeadpatCount, error) {
	ctx := context.Background()

	pendingKey := "event:headpat:pending"
	pending, err := s.client.Get(ctx, pendingKey).Int()
	if err != nil && err != redis.Nil {
		return HeadpatCount{}, err
	}

	totalKey := "event:headpat:total"
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

func (s *redisStore) GetHeadpatCount() (HeadpatCount, error) {
	ctx := context.Background()

	pendingKey := "event:headpat:pending"
	pending, err := s.client.Get(ctx, pendingKey).Int()
	if err != nil && err != redis.Nil {
		return HeadpatCount{}, err
	}

	totalKey := "event:headpat:total"
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

func (s *redisStore) AddStreamStartEvent(streamId string, timestamp time.Time) error {
	ctx := context.Background()
	setKey := "stream:online"
	entry := redis.Z{Member: streamId, Score: float64(timestamp.Unix())}
	return s.client.ZAdd(ctx, setKey, entry).Err()
}

func (s *redisStore) AddOutOfStockEvent(streamId string, timestamp time.Time) error {
	ctx := context.Background()
	setKey := "stream:out-ouf-stock"
	entry := redis.Z{Member: streamId, Score: float64(timestamp.Unix())}
	return s.client.ZAdd(ctx, setKey, entry).Err()
}

func (s *redisStore) IsOutOfStock() bool {
	ctx := context.Background()
	setKey := "stream:out-ouf-stock"
	ts := time.Now().Add(-15 * time.Minute)
	min := fmt.Sprintf("(%d", ts.Unix())
	result := s.client.ZCount(ctx, setKey, min, "+inf").Val()
	return result > 0
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
