# Kestrel Equipment Rental

A rental management system for tools and industrial machines — jackhammers, paint
sprayers, high-capacity pumps and the rest of the yard.

It covers the whole cycle: a customer books online, staff hand the machine over at
the counter, the system chases the return, and it invoices for late fees and damage
when it comes back.

## The three interfaces

| Who | Where | What they do |
|---|---|---|
| **Customers** | `/` and `/account` | Browse by date, see live availability, book and pay, track rentals, settle invoices |
| **Counter staff** | `/desk` | The day's collections, returns and overdue list; hand over specific units; take equipment back and settle up; register walk-ins |
| **Administrators** | `/admin` | Catalog and rates, the physical fleet, people and roles, the email log, the job queue |

Role is a property of the account — one sign-in page, and the navigation shows what
that person is allowed to reach.

## Architecture

```
browser ──► frontend (nginx)  ──/api──►  api (Go)  ──►  db (Postgres)
                 React SPA                  │
                                            ▼
                                     jobs table  ◄──  worker (Go)  ──►  mailpit
```

- **`backend/`** — Go. Three binaries from one image: `api` (REST), `worker`
  (reminder emails, overdue sweep), `seed` (synthetic data). Dependencies are
  `pgx` and its transitives, and nothing else: routing is stdlib `net/http`,
  password hashing is stdlib `crypto/pbkdf2`, migrations are ~60 lines of our own.
- **`frontend/`** — React 19 + TypeScript + Vite. Three dependencies: React,
  React DOM, React Router. Hand-written CSS design system, no UI framework.
- **`stack/`** — the deployment definition, described below.

The API is versioned under `/api/v1` and knows nothing about the frontend, so a
mobile or white-label client is just another consumer of the same endpoints.

### Some decisions worth knowing

**Money is integer cents everywhere.** It becomes a decimal only when it is
displayed. `internal/money` also owns the rate rules and is unit-tested.

**Rates pick themselves.** A rental is charged on whichever of the daily, weekly
or monthly rate is cheapest for the length booked — nine days bills as two weeks,
not nine days. The customer never pays more by hiring for longer.

**Availability is peak concurrent demand**, computed per day across the requested
window, not a sum of overlapping bookings. Two back-to-back rentals in the same
week do not each consume a unit for the whole week.

**Overbooking is prevented under concurrency.** Booking re-checks availability
inside the transaction under a per-model advisory lock, taken in a stable order,
so two customers racing for the last jackhammer cannot both win.

**A booking is a model; a unit is assigned at the counter.** Customers reserve
"a 14 lb breaker"; staff choose which physical asset goes out, and it is metered,
tracked and inspected on return.

**Payment failure never loses a booking.** The reservation is committed first;
the card is charged after. A decline leaves a real reservation with an unpaid
invoice — which is what a counter would do.

**The job queue is a Postgres table** claimed with `FOR UPDATE SKIP LOCKED`. No
broker, no Redis. Reminders are deduplicated on the reservation, so rescheduling
moves them rather than duplicating them.

**Payments and email are interfaces.** `billing.Gateway` has a working fake that
declines and expires realistically; `notify.Mailer` speaks SMTP. Swapping in
Stripe or a hosted mail API means implementing one interface each.

## Running it

Everything runs as a [`stack`](https://github.com/bozemanpass/stack), so the same
definition deploys to Docker Compose locally and to Kubernetes unchanged.

```bash
stack build containers --stack ./stack

stack init --stack ./stack --output rental-spec.yml \
  --deploy-to compose --map-ports-to-host localhost-same \
  --config PUBLIC_BASE_URL=http://localhost:8088 \
  --config CORS_ORIGINS=http://localhost:8088

stack deploy --spec-file rental-spec.yml --deployment-dir ./rental-deployment
stack manage --dir ./rental-deployment start
```

Then open **http://localhost:8088**. Mailpit's inbox is on **http://localhost:8025** —
every message the system sends lands there.

> The generated spec maps the frontend to host port 80, which needs root. Edit the
> `network.ports` block to `8088:80` and `8089:8080` before deploying, as above.

Useful afterwards:

```bash
stack manage --dir ./rental-deployment status
stack manage --dir ./rental-deployment logs -f worker
stack manage --dir ./rental-deployment stop
```

### Deploying somewhere real

The same stack, one flag different:

```bash
stack init --stack ./stack --output k8s-spec.yml --deploy-to k8s \
  --image-registry registry.example.com \
  --http-proxy-fqdn rentals.example.com --http-proxy-target frontend:80
```

For a plain VM, Compose is the right target — see
[from-laptop-to-production](https://github.com/bozemanpass/stack/blob/main/docs/from-laptop-to-production.md).

Before any real deployment: set `SEED_ON_BOOT=false`, point `SMTP_HOST` at a real
relay and drop the `mailpit` service, and replace the fake payment gateway in
`internal/app/app.go` with a real one.

## Development

The stack is the easiest way to run the whole system, but the loop is faster with
the apps running natively against containerized infrastructure:

```bash
# Postgres and Mailpit only
docker run -d --name rentals-db   -e POSTGRES_PASSWORD=devpass -e POSTGRES_DB=rentals -p 5432:5432 postgres:17-alpine
docker run -d --name rentals-mail -p 1025:1025 -p 8025:8025 axllent/mailpit

export DATABASE_URL="postgres://postgres:devpass@localhost:5432/rentals?sslmode=disable"
export SMTP_HOST=localhost SMTP_PORT=1025

cd backend
go run ./cmd/seed --reset     # migrate and fill with synthetic data
go run ./cmd/api              # :8080
go run ./cmd/worker           # in another shell

cd ../frontend
npm install && npm run dev    # :5173, proxies /api to :8080
```

```bash
cd backend  && go test ./... && go vet ./...
cd frontend && npm run typecheck && npm run build
```

## The synthetic data

`cmd/seed` builds a believable yard so every screen has something real to show
from the first run. It is deterministic — the same random seed always produces
the same data — and refuses to run against a non-empty database unless given
`--reset`.

It generates 8 categories, 31 equipment models with real specifications and
rates, ~160 physical units (a few deliberately in maintenance or retired), 24
customers, 3 counter staff, an administrator, and around 70 rentals spread
across **every** state the system can be in: completed, cancelled, out on hire,
overdue, due back today, being collected today, and booked for the future — with
matching invoices, payments, deposit holds and a backfilled email log.

The API can also seed itself on first boot with `SEED_ON_BOOT=true`, which is how
a fresh `stack` deployment arrives with data already in it.

### Signing in

Every seeded account uses the password `rentals123`.

| Role | Email |
|---|---|
| Administrator | `admin@kestrelrental.example` |
| Counter staff | `marisol@kestrelrental.example` |
| Customer | `dana.whitfield@example.com` |

The sign-in page lists these and fills them in on click. That is demonstration
scaffolding — it has no place in a real deployment, and a real deployment is
never seeded.

### Test cards

The payment gateway is a stand-in that behaves like a real processor:

| Number | Result |
|---|---|
| `4242 4242 4242 4242` | Succeeds |
| `4000 0000 0000 0002` | Declined — insufficient funds |
| `4000 0000 0000 0069` | Declined — expired card |

## How the money works

1. **At booking** the rental is invoiced and the deposit is authorized as a hold.
2. **At the counter** staff assign physical units and record meter readings.
3. **On return** each unit is inspected. Late fees (daily rate × days late × 1.5)
   and any damage are billed on a second invoice, so the original stays a clean
   record of what was agreed.
4. **The deposit** covers what is owed; whatever is left is released.

Tax rate, late-fee multiple and reminder lead times are all environment
configuration — see `internal/config/config.go`.
