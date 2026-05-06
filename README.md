# comms-demo (mock-channels)

A standalone Go webapp that pretends to be three messaging providers — **email**, **Slack**, and **SMS** — so Helix Org can be demoed end-to-end without hitting real third-party services.

It is **not** a real provider integration. There is **no auth, no signing, no replay protection**. Public demo.

## Wire spec

Helix Org's webhook transport is the contract. Two directions:

- **Helix Org → mock app** — Helix POSTs `application/json` with a `domain.Message` body to `POST /in/<channel_id>`. We respond `204`.
- **Mock app → Helix Org** — operator clicks Reply / Compose; we POST the same `Message` envelope to `<helix_base_url>/webhooks/<helix_stream_id>`.

Threading discipline (mirrors a real provider):

- First message in a thread: `thread_id == message_id`.
- Reply: copy `thread_id` from parent; `in_reply_to = parent.message_id`.

## Run

```bash
make build
./bin/mock-channels serve --addr :7765 --db ./mock-channels.db
```

Open <http://localhost:7765/> for the three-pane phone view. <http://localhost:7765/admin> to add or edit channels.

Seed defaults (point at a Helix Org base URL):

```bash
./bin/mock-channels seed \
  --helix-base http://localhost:8080 \
  --email-stream strm-email \
  --slack-stream strm-slack \
  --sms-stream  strm-sms
```

Reset:

```bash
./bin/mock-channels reset
```

## Demo flow

1. Run `helix-org bootstrap` and `helix-org serve`.
2. Run `mock-channels serve` on a host Helix can reach.
3. Create three webhook Streams in Helix Org with `outbound_url` pointing at `http(s)://<mock-host>/in/<channel_id>` for each kind.
4. Run `mock-channels seed` (or use `/admin`) to register the matching `(channel_id, helix_stream_id)` rows.
5. A Helix Worker publishes a message → mock-channels receives it → renders in the appropriate channel UI.
6. Operator clicks Reply → mock-channels POSTs to `/webhooks/<streamID>` → Helix Org appends the event.
