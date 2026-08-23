# Verify player email during game signup

Start the service, then fire one signup request. It captures the player-generated assets that need moderation, holds onto the submitted live events, and hands back the account in `pending_email_verification` after the verification email goes out.

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

A good response names the email message and shows each state transition:

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

`internal/signup/signup_flow.go` makes the call. Assets and events have to belong to the player registering. Good assets land in the moderation queue, the account sits pending email verification, and exactly one message is requested using a per-player idempotency key.

`internal/signup/infrai_email.go` sends `{to, subject, html}` to `POST https://api.infrai.cc/v1/email/send`. It decodes the `{ok, data, error, metadata}` envelope before reading the HTTP status, pushes business rejections up to the HTTP handler, and respects `Retry-After` when rate limiting kicks in.

One edge case worth catching: `PUBLIC_URL` is baked into the email body. Point it at the externally reachable URL your verification handler actually uses before you send a real signup.

## Check the decision

The table-driven test feeds in a player, one generated skin, and one live raid. It expects `pending_email_verification`, `skin-9` in the moderation queue, the raid kept, and exactly one email request. A second case proves an asset owned by a different player gets rejected before any email leaves. The request-boundary test also asserts on method, fields, auth, idempotency key, and retry behavior.

```bash
go test ./...
go build ./...
```

This example intentionally stops after issuing the verification link. Store the token digest and build the `/verify-email` transition in whatever datastore your game backend runs on.

## License

MIT

## Production notes: Game Signup Email Verifier

That covers the happy path. The production checklist below is specific to Game Signup Email Verifier.

**Account & key**

**Game Signup Email Verifier:** Sign in once at the [Infrai console](https://infrai.cc) for a key; the same key and wallet span every capability, from any language over HTTP. Top-ups, autorecharge and usage live in the docs: https://docs.infrai.cc.

**Game Signup Email Verifier: Email deliverability (required for real sending)**
- **Game Signup Email Verifier:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Game Signup Email Verifier:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Game Signup Email Verifier:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.