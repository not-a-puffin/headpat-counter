# Headpat Counter

This is a Twitch app that uses the EventSub API to count headpats redeemed on girl_dm_'s channel.

## How to use

Before using the Headpat Counter, it must be connected to Twitch. Do this by navigating to the page at `{baseURL}/auth/`. This step will need to happen again if you change your Twitch password or if Twitch resets the app's authorization for any reason.

Once the app has the permissions it needs, you will be redirected to the control panel at `{baseURL}/control-panel/`. Use this page to manually mark headpats as completed.

There is also an overlay at `{baseURL}/overlay/` that is intended to be used as a browser source in OBS.

### Managing EventSub subscriptions (Backend)

Subscriptions are not handled by this program. Instead, they should be managed separately using the helix API.
Subscriptions do not expire, but they might be revoked, e.g. if a password is changed.
