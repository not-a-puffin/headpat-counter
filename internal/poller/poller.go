package poller

import (
	"context"
	"headpat-counter/internal/config"
	"headpat-counter/internal/request"
	"headpat-counter/internal/store"
	"log"
	"time"
)

var CurrentStreamId string

type HeadpatPoller struct {
	cfg      *config.AppConfig
	st       store.Store
	streamId string
	token    string
}

func NewHeadpatPoller(cfg *config.AppConfig, st store.Store, streamId string) HeadpatPoller {
	// Refresh user access token
	var token string
	tokenPair, _ := st.GetTokenPair("user")
	if tokenPair != nil {
		tokenResult, _ := request.RefreshUserAccessToken(cfg, tokenPair.Refresh)
		if tokenResult != nil {
			token = tokenResult.AccessToken
			tokenPair := store.TokenPair{
				Access:  tokenResult.AccessToken,
				Refresh: tokenResult.RefreshToken,
			}
			if err := st.SetTokenPair("user", tokenPair); err != nil {
				log.Printf("Error: failed to save user access token: %s\n", err)
			}
		}
	}

	return HeadpatPoller{cfg: cfg, st: st, streamId: streamId, token: token}
}

func (p *HeadpatPoller) doPoll() bool {
	if CurrentStreamId != p.streamId {
		log.Println("Stopping headpat poller due to old stream ID")
		return false
	}

	if p.token == "" {
		log.Println("Stopping headpat poller due to missing user access token")
		return false
	}

	reward, err := request.GetHeadpatReward(p.cfg, p.token)
	if err != nil {
		log.Printf("Error occurred while polling headpats: %s\n", err)
	}
	if reward != nil {
		if reward.RedemptionsRedeemedCurrentStream == nil {
			log.Printf("HeadpatPoller(streamId: %s) Status: { Redeemed: nil, Out of Stock: %t }\n", p.streamId, !reward.IsInStock)
		} else {
			count := *reward.RedemptionsRedeemedCurrentStream
			log.Printf("HeadpatPoller(streamId: %s) Status: { Redeemed: %v, Out of Stock: %t }\n", p.streamId, count, !reward.IsInStock)
			p.st.AddNumRedeemedThisStream(p.streamId, count)
			if count > 0 && !reward.IsInStock {
				log.Println("Headpats out of stock!")
				timestamp := string(time.Now().Format(time.RFC3339Nano))
				p.st.AddOutOfStockEvent(p.streamId, timestamp)
				return false
			}
		}
	}
	return true
}

func (p *HeadpatPoller) Start(ctx context.Context) {
	log.Printf("HeadpatPoller(streamId: %s) Starting\n", p.streamId)

	ticker := time.NewTicker(p.cfg.PollerFrequency)
	defer ticker.Stop()

	shouldContinue := p.doPoll()

	for shouldContinue {
		select {
		case <-ticker.C:
			shouldContinue = p.doPoll()
		case <-ctx.Done():
			goto finish
		}
	}

finish:
	log.Printf("HeadpatPoller(streamId: %s) Finished\n", p.streamId)
}
