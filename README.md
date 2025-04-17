# Twitch EventSub Headpat Counter

This app uses the new EventSub API to count headpats.

Using the EventSub API requires a server that can respond to http requests using TLS/SSL.

## How to use

### Create a `.env` file

Create a file with the following variables:
```bash
# .env
PORT=3000
WEBHOOK_SECRET=...
```

### Register the app

See the documentation for [registering your app](https://dev.twitch.tv/docs/authentication/register-app/).

You will need to use your __Client ID__ and an __App Access Token__ when creating EventSub subscriptions.

You can get an app access token using the [client credentials grant flow](https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/#client-credentials-grant-flow).

An app token should be valid for around 60 days. If a token expires, you will have to get a new one.

```bash
# Example using the client credential app flow
curl -X POST 'https://id.twitch.tv/oauth2/token?client_id=CLIENT_ID&client_secret=CLIENT_SECRET&grant_type=client_credentials'

# Response:
{
  "access_token": "jostpf5q0uzmxmkba9iyug38kjtgh",
  "expires_in": 5011271,
  "token_type": "bearer"
}
```

### Authorize scopes

In order to subscribe to channel point redemption events, the broadcaster must authorize your app for the `channel:read:redemptions` scope.

Follow the instructions for the [authorization code grant flow](https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/#authorization-code-grant-flow).

This flow expects a redirect uri to send the authorization code to. As far as I can tell, we can ignore this part, since we do not need to store the code anywhere - it just needs to exist on Twitch's end.

```bash
# Example URI
# Broadcaster should navigate here to authorize the channel:read:redemptions scope for your app.
https://id.twitch.tv/oauth2/authorize
    ?response_type=code
    &client_id=CLIENT_ID
    &redirect_uri=BASE_URL/auth
    &scope=channel%3Aread%3Aredemptions
```

### Building and running the server

The build script uses [UPX](https://upx.github.io/) for a smaller executable.

The server automatically logs some events. You may want to ignore these logs or redirect them to a file.

```bash
# run build script
./build.sh

# start the server and log to standard out
./bin/server

# or pipe logs to text file
./bin/server >> ./log/log.txt 2>&1

# or ignore logs
./bin/server >& /dev/null
```

### Managing subscriptions

Subscriptions are not handled by this program. Instead, they should be managed externally using the helix API.

Here are some examples using __curl__ to set up EventSub subscriptions:

```bash
# LIST SUBSCRIPTIONS
curl -X GET 'https://api.twitch.tv/helix/eventsub/subscriptions' \
-H 'Authorization: Bearer APP_ACCESS_TOCKEN' \
-H 'Client-Id: CLIENT_ID'

# CREATE STREAM.ONLINE SUBSCRIPTION
curl -X POST 'https://api.twitch.tv/helix/eventsub/subscriptions' \
-H 'Authorization: Bearer APP_ACCESS_TOCKEN' \
-H 'Client-Id: CLIENT_ID' \
-H 'Content-Type: application/json' \
-d '{"type":"stream.online","version":"1","condition":{"broadcaster_user_id":"USER_ID"},"transport":{"method":"webhook","callback":"BASE_URL/notification","secret":"WEBHOOK_SECRET"}}'

# CREATE STREAM.OFFLINE SUBSCRIPTION
curl -X POST 'https://api.twitch.tv/helix/eventsub/subscriptions' \
-H 'Authorization: Bearer APP_ACCESS_TOCKEN' \
-H 'Client-Id: CLIENT_ID' \
-H 'Content-Type: application/json' \
-d '{"type":"stream.offline","version":"1","condition":{"broadcaster_user_id":"USER_ID"},"transport":{"method":"webhook","callback":"BASE_URL/notification","secret":"WEBHOOK_SECRET"}}'

# CREATE CHANNEL.CHANNEL_POINTS_CUSTOM_REWARD_REDEMPTION.ADD SUBSCRIPTION
curl -X POST 'https://api.twitch.tv/helix/eventsub/subscriptions' \
-H 'Authorization: Bearer APP_ACCESS_TOCKEN' \
-H 'Client-Id: CLIENT_ID' \
-H 'Content-Type: application/json' \
-d '{"type":"channel.channel_points_custom_reward_redemption.add","version":"1","condition":{"broadcaster_user_id":"USER_ID", "reward_id":"REWARD_ID"},"transport":{"method":"webhook","callback":"BASE_URL/notification","secret":"WEBHOOK_SECRET"}}'

# DELETE SUBSCRIPTION
curl -X DELETE 'https://api.twitch.tv/helix/eventsub/subscriptions?id=SUBSCRIPTION_ID' \
-H 'Authorization: Bearer APP_ACCESS_TOCKEN' \
-H 'Client-Id: CLIENT_ID'
```
