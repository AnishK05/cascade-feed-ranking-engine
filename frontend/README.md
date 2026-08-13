# Frontend

Next.js App Router demo UI for Cascade. Talks **only** to the Gateway
(`NEXT_PUBLIC_API_BASE`, default `http://localhost:8080`).

You do not need to run this separately. `make up` / `cascade.cmd up` builds and
serves it on http://localhost:3000.

## Host-native dev (optional)

```bash
npm ci
npm run dev
```

Requires the Gateway on :8080. The Compose image bakes `NEXT_PUBLIC_API_BASE` at
**build** time — rebuild the `frontend` service if you change it.

Pages: `/feed`, `/graph`, `/admin`. User switcher sends `X-User-Id`.
Walkthrough: [`docs/demo.md`](../docs/demo.md).
