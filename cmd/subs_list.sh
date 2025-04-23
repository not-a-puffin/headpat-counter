#!/bin/bash

set -euo pipefail

export $(cat .env | xargs)

curl -w "\n" -X GET 'https://api.twitch.tv/helix/eventsub/subscriptions' \
-H 'Authorization: Bearer '$APP_ACCESS_TOKEN'' \
-H 'Client-Id: '$APP_CLIENT_ID''
