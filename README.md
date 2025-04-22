# Headpat Counter

This is a Twitch app that uses the EventSub API to count headpats redeemed on girl_dm_'s channel.

## How to use

Before using the Headpat Counter, it must be connected to Twitch. Do this by navigating to the page at `{baseURL}/auth/`. This step will need to happen again if you change your Twitch password or if Twitch resets the app's authorization for any reason.

![Image](https://github.com/user-attachments/assets/6017f6f9-b624-4640-89db-8c9ea8778bac)

Once the app has the permissions it needs, you will be redirected to the control panel at `{baseURL}/control-panel/`. Use this page to manually mark headpats as completed.

![Image](https://github.com/user-attachments/assets/ea420ce9-ef41-4a09-ab5d-e504f00a7fee)

There is also an overlay at `{baseURL}/overlay/` that is intended to be used as a browser source in OBS.

![Image](https://github.com/user-attachments/assets/942c5e21-0a73-4ac9-bb3a-804f156b4d70)

### Managing EventSub subscriptions (Backend)

Subscriptions are not handled by this program. Instead, they should be managed separately using the helix API.
Subscriptions do not expire, but they might be revoked, e.g. if a password is changed.
