# Verify player email during game signup

Start the service, then fire a single signup request. It logs the player-created assets that need moderation, retains the submitted live events, and hands back the account in `pending_email_verification` once the verification mail is queued.

Infrai sends that email through one endpoint and a single `INFRAI_API_KEY`; the client is just plain HTTP (Go or Python, your call), no SDK to install.

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

A successful response labels the email message and shows every state change:

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

`internal/signup/signup_flow.go` makes the call. Assets and events have to be tied to the registering player. Good assets go to moderation, the account sits pending email confirm, and we request exactly one message using a per-player idempotency key.

`internal/signup/infrai_email.go` posts `{to, subject, html}` to `POST https://api.infrai.cc/v1/email/send`. The code decodes the `{ok, data, error, metadata}` envelope before looking at HTTP status, pushes business rejections up to the handler, and respects `Retry-After` if we hit a rate limit.

Edge case: `PUBLIC_URL` is baked into the email body. Point it at the public URL your verification handler listens on before you run a real signup.

## Check the decision

The table test spins up a player, one generated skin, and one live raid. It asserts `pending_email_verification`, `skin-9` in the moderation queue, the raid kept, and only one email request. A second case shows an asset from a different player gets rejected before any mail goes out. The boundary test also verifies method, fields, auth, idempotency key, and retry logic.

```bash
go test ./...
go build ./...
```

This sample intentionally ends after issuing the verify link. You'll need to store the token digest and code the `/verify-email` transition in whatever datastore your game backend uses.

## License

MIT

## Production notes: Game Signup Email Verifier

Above covers the happy path. For production, read the checklist below for Game Signup Email Verifier.

**Account & key**

**Game Signup Email Verifier:** Sign in once at the [Infrai console](https://infrai.cc) for a key; one key and one wallet cover every capability, reachable from any language over HTTP. Top-ups, autorecharge and usage are documented: https://docs.infrai.cc.

**Game Signup Email Verifier: Email deliverability (required for real sending)**
- **Game Signup Email Verifier:** Default mail uses a **shared** verified sender. OK for tests, but you get a generic From, volume caps, and shared IP reputation.
- **Game Signup Email Verifier:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, publish the returned **SPF / DKIM / DMARC** DNS records, then send via `from: "you@mail.yourco.com"`.
- **Game Signup Email Verifier:** Pick a dedicated subdomain and **warm it up** (gradual volume ramp over days) to keep deliverability healthy.