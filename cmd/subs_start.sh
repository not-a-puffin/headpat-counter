#!/bin/bash

set -euo pipefail

export $(cat .env | xargs)

curl -w "\n" -X POST 'https://api.twitch.tv/helix/eventsub/subscriptions' \
-H 'Authorization: Bearer '$APP_ACCESS_TOKEN'' \
-H 'Client-Id: '$APP_CLIENT_ID'' \
-H 'Content-Type: application/json' \
-d '{"type":"channel.channel_points_custom_reward_redemption.add","version":"1","condition":{"broadcaster_user_id":"'$BROADCASTER_USER_ID'", "reward_id":"'$REWARD_ID'"},"transport":{"method":"webhook","callback":"'$BASE_URL'/headpat/notification","secret":"'$WEBHOOK_SECRET'"}}'
