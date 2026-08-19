
CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    email           TEXT NOT NULL UNIQUE,
    password_hash   TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('customer','staff','admin')),
    full_name       TEXT NOT NULL,
    phone           TEXT NOT NULL DEFAULT '',
    company         TEXT NOT NULL DEFAULT '',
    address_line1   TEXT NOT NULL DEFAULT '',
    address_line2   TEXT NOT NULL DEFAULT '',
    city            TEXT NOT NULL DEFAULT '',
    state           TEXT NOT NULL DEFAULT '',
    postal_code     TEXT NOT NULL DEFAULT '',
    license_number  TEXT NOT NULL DEFAULT '',
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX users_role_idx ON users (role);

CREATE TABLE sessions (
    token_hash  TEXT PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expiry_idx ON sessions (expires_at);

CREATE TABLE categories (
    id          BIGSERIAL PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order  INT NOT NULL DEFAULT 0
);

-- A rentable product: "14lb Electric Jackhammer". Customers reserve one of these.
CREATE TABLE equipment_models (
    id                      BIGSERIAL PRIMARY KEY,
    category_id             BIGINT NOT NULL REFERENCES categories(id),
    sku                     TEXT NOT NULL UNIQUE,
    name                    TEXT NOT NULL,
    description             TEXT NOT NULL DEFAULT '',
    manufacturer            TEXT NOT NULL DEFAULT '',
    daily_rate_cents        BIGINT NOT NULL CHECK (daily_rate_cents >= 0),
    weekly_rate_cents       BIGINT NOT NULL CHECK (weekly_rate_cents >= 0),
    monthly_rate_cents      BIGINT NOT NULL CHECK (monthly_rate_cents >= 0),
    deposit_cents           BIGINT NOT NULL DEFAULT 0 CHECK (deposit_cents >= 0),
    replacement_value_cents BIGINT NOT NULL DEFAULT 0,
    requires_license        BOOLEAN NOT NULL DEFAULT FALSE,
    specs                   JSONB NOT NULL DEFAULT '{}'::jsonb,
    image_url               TEXT NOT NULL DEFAULT '',
    active                  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX equipment_models_category_idx ON equipment_models (category_id);

-- A physical machine on the yard, assigned to a reservation at pickup time.
CREATE TABLE equipment_units (
    id              BIGSERIAL PRIMARY KEY,
    model_id        BIGINT NOT NULL REFERENCES equipment_models(id),
    asset_tag       TEXT NOT NULL UNIQUE,
    serial_number   TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'available'
                    CHECK (status IN ('available','out','maintenance','retired')),
    condition_notes TEXT NOT NULL DEFAULT '',
    meter_hours     NUMERIC(10,1) NOT NULL DEFAULT 0,
    acquired_on     DATE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX equipment_units_model_idx ON equipment_units (model_id);
CREATE INDEX equipment_units_status_idx ON equipment_units (status);

CREATE TABLE reservations (
    id                  BIGSERIAL PRIMARY KEY,
    reservation_number  TEXT NOT NULL UNIQUE,
    customer_id         BIGINT NOT NULL REFERENCES users(id),
    status              TEXT NOT NULL DEFAULT 'confirmed'
                        CHECK (status IN ('confirmed','picked_up','returned','cancelled')),
    start_date          DATE NOT NULL,
    end_date            DATE NOT NULL,
    picked_up_at        TIMESTAMPTZ,
    returned_at         TIMESTAMPTZ,
    subtotal_cents      BIGINT NOT NULL DEFAULT 0,
    tax_cents           BIGINT NOT NULL DEFAULT 0,
    deposit_cents       BIGINT NOT NULL DEFAULT 0,
    total_cents         BIGINT NOT NULL DEFAULT 0,
    notes               TEXT NOT NULL DEFAULT '',
    created_by          BIGINT REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date >= start_date)
);
CREATE INDEX reservations_customer_idx ON reservations (customer_id);
CREATE INDEX reservations_status_idx ON reservations (status);
CREATE INDEX reservations_dates_idx ON reservations (start_date, end_date);

CREATE TABLE reservation_items (
    id                  BIGSERIAL PRIMARY KEY,
    reservation_id      BIGINT NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    model_id            BIGINT NOT NULL REFERENCES equipment_models(id),
    quantity            INT NOT NULL CHECK (quantity > 0),
    rate_basis          TEXT NOT NULL CHECK (rate_basis IN ('daily','weekly','monthly')),
    rate_cents          BIGINT NOT NULL,
    billable_periods    INT NOT NULL,
    line_total_cents    BIGINT NOT NULL
);
CREATE INDEX reservation_items_reservation_idx ON reservation_items (reservation_id);
CREATE INDEX reservation_items_model_idx ON reservation_items (model_id);

-- Which physical unit went out against which line, and in what shape it came back.
CREATE TABLE unit_assignments (
    id                  BIGSERIAL PRIMARY KEY,
    reservation_item_id BIGINT NOT NULL REFERENCES reservation_items(id) ON DELETE CASCADE,
    unit_id             BIGINT NOT NULL REFERENCES equipment_units(id),
    checked_out_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    checked_in_at       TIMESTAMPTZ,
    checkout_notes      TEXT NOT NULL DEFAULT '',
    checkin_notes       TEXT NOT NULL DEFAULT '',
    damage_cents        BIGINT NOT NULL DEFAULT 0,
    meter_out           NUMERIC(10,1),
    meter_in            NUMERIC(10,1)
);
CREATE UNIQUE INDEX unit_assignments_open_unit_idx
    ON unit_assignments (unit_id) WHERE checked_in_at IS NULL;
CREATE INDEX unit_assignments_item_idx ON unit_assignments (reservation_item_id);

CREATE TABLE invoices (
    id                  BIGSERIAL PRIMARY KEY,
    invoice_number      TEXT NOT NULL UNIQUE,
    reservation_id      BIGINT NOT NULL REFERENCES reservations(id),
    customer_id         BIGINT NOT NULL REFERENCES users(id),
    status              TEXT NOT NULL DEFAULT 'issued'
                        CHECK (status IN ('draft','issued','paid','void')),
    issued_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    due_at              TIMESTAMPTZ NOT NULL,
    subtotal_cents      BIGINT NOT NULL DEFAULT 0,
    tax_cents           BIGINT NOT NULL DEFAULT 0,
    total_cents         BIGINT NOT NULL DEFAULT 0,
    amount_paid_cents   BIGINT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX invoices_customer_idx ON invoices (customer_id);
CREATE INDEX invoices_reservation_idx ON invoices (reservation_id);
CREATE INDEX invoices_status_idx ON invoices (status);

CREATE TABLE invoice_lines (
    id                  BIGSERIAL PRIMARY KEY,
    invoice_id          BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    kind                TEXT NOT NULL CHECK (kind IN ('rental','late_fee','damage','deposit','discount')),
    description         TEXT NOT NULL,
    quantity            INT NOT NULL DEFAULT 1,
    unit_amount_cents   BIGINT NOT NULL,
    amount_cents        BIGINT NOT NULL,
    sort_order          INT NOT NULL DEFAULT 0
);
CREATE INDEX invoice_lines_invoice_idx ON invoice_lines (invoice_id);

CREATE TABLE payments (
    id              BIGSERIAL PRIMARY KEY,
    invoice_id      BIGINT REFERENCES invoices(id),
    reservation_id  BIGINT REFERENCES reservations(id),
    customer_id     BIGINT NOT NULL REFERENCES users(id),
    kind            TEXT NOT NULL CHECK (kind IN ('authorization','capture','refund','release')),
    amount_cents    BIGINT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('pending','succeeded','failed')),
    gateway         TEXT NOT NULL DEFAULT 'fake',
    gateway_ref     TEXT NOT NULL DEFAULT '',
    card_brand      TEXT NOT NULL DEFAULT '',
    card_last4      TEXT NOT NULL DEFAULT '',
    failure_reason  TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX payments_invoice_idx ON payments (invoice_id);
CREATE INDEX payments_customer_idx ON payments (customer_id);

-- Durable job queue driving reminder emails. Claimed with FOR UPDATE SKIP LOCKED.
CREATE TABLE jobs (
    id              BIGSERIAL PRIMARY KEY,
    kind            TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}'::jsonb,
    dedupe_key      TEXT UNIQUE,
    run_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','running','done','failed')),
    attempts        INT NOT NULL DEFAULT 0,
    last_error      TEXT NOT NULL DEFAULT '',
    locked_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX jobs_claim_idx ON jobs (status, run_at);

-- Every message the system produced, whatever transport actually delivered it.
CREATE TABLE emails (
    id              BIGSERIAL PRIMARY KEY,
    to_address      TEXT NOT NULL,
    to_name         TEXT NOT NULL DEFAULT '',
    subject         TEXT NOT NULL,
    template        TEXT NOT NULL,
    body_text       TEXT NOT NULL,
    body_html       TEXT NOT NULL DEFAULT '',
    reservation_id  BIGINT REFERENCES reservations(id),
    status          TEXT NOT NULL CHECK (status IN ('sent','failed')),
    error           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX emails_reservation_idx ON emails (reservation_id);
CREATE INDEX emails_created_idx ON emails (created_at DESC);


-- +down
DROP TABLE IF EXISTS emails, jobs, payments, invoice_lines, invoices,
    unit_assignments, reservation_items, reservations, equipment_units,
    equipment_models, categories, sessions, users CASCADE;
