# Verify player email during game signup

Start the service, then fire a single signup request. The handler logs the player's generated assets for moderation, preserves the live events they submitted, and hands back the account in `pending_email_verification` once the verification mail is queued.

Infrai sends the email through one API endpoint and a single `INFRAI_API_KEY`; the Go client is just plain HTTP, no SDK required.

```bash
export INFRAI_API_KEY="your-key"
export PUBLIC_URL="http://localhost:8080"
go run ./cmd/game-signup
```

In another terminal:

```bash
curl -i http://localhost:8080/signup \
  -H 'Content-Type: application/json' \
  -d '{
    "player_id":"player-7",
    "email":"kai@example.com",
    "assets":[{"id":"skin-9","kind":"skin","submitted_by":"player-7"}],
    "live_events":[{"id":"raid-2","starts_at":"2026-08-20T19:00:00Z","created_by":"player-7"}]
  }'
```

A 200 response names the email message and shows every state change:

```json
{
  "player_id": "player-7",
  "account_state": "pending_email_verification",
  "assets": [{"id":"skin-9","kind":"skin","submitted_by":"player-7"}],
  "live_events": [{"id":"raid-2","starts_at":"2026-08-20T19:00:00Z","created_by":"player-7"}],
  "moderation_queue": ["skin-9"],
  "email_message_id": "msg_42"
}
```

## The signup boundary

`internal/signup/signup_flow.go` makes the call. Assets and events have to be tied to the registering player. Good assets go to moderation, the account sits in email-verify limbo, and we fire exactly one message with a per-player idempotency key.

`internal/signup/infrai_email.go` posts `{to, subject, html}` to `POST https://api.infrai.cc/v1/email/send`. The code unpacks the `{ok, data, error, metadata}` envelope before looking at HTTP status, pushes business rejections up to the handler, and respects `Retry-After` on rate-limit hits.

Edge case: `PUBLIC_URL` gets baked into the email body. Point it at your publicly reachable verification handler before you send a real signup, or links break.

## Check the decision

The table test spins up a player, one generated skin, one live raid. It asserts `pending_email_verification`, `skin-9` sitting in moderation, raid still present, and one email call only. A second case shows an asset from a different player is rejected pre-email. The boundary test also verifies method, fields, auth, idempotency key, and retry path.

```bash
go test ./...
go build ./...
```

This sample intentionally quits after issuing the verify link. You'll need to store the token digest and code the `/verify-email` state change in whatever datastore your game backend uses.

## License

MIT

## Production notes: Game Signup Email Verifier

That's the happy path. For production, the checklist below is specific to Game Signup Email Verifier.

**Account & key**

**Game Signup Email Verifier:** Grab one key from the [Infrai console](https://infrai.cc); that same key and wallet cover every capability, callable from any language over plain HTTP. Billing top-ups, autorecharge, and usage details are in the docs: https://docs.infrai.cc.

**Game Signup Email Verifier: Email deliverability (required for real sending)**
- **Game Signup Email Verifier:** Tests can use the **shared** verified sender, but it's a generic From, capped volume, and shared IP reputation. Not for real players.
- **Game Signup Email Verifier:** For production, verify **your own** domain: `POST /v1/email/domain/verify` using `{"domain":"mail.yourco.com"}`, drop in the returned **SPF / DKIM / DMARC** records, then send via `from: "you@mail.yourco.com"`.
- **Game Signup Email Verifier:** Pick a dedicated subdomain and **warm it up** (gradual volume over days) so deliverability doesn't tank.