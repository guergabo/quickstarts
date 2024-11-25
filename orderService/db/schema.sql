DROP TYPE IF EXISTS ORDER_STATUS CASCADE;
DROP TYPE IF EXISTS OUTBOX_STATUS CASCADE;

DROP TABLE IF EXISTS order_outboxes;
DROP TABLE IF EXISTS orders; --remove for antithesis

CREATE TYPE ORDER_STATUS AS ENUM (
    'pending', 
    'succeeded', 
    'failed'
); 

CREATE TYPE OUTBOX_STATUS AS ENUM (
    'pending', 
    'succeeded', 
    'failed'
); 

CREATE TABLE IF NOT EXISTS orders (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    amount      NUMERIC(10, 2) NOT NULL CONSTRAINT amount_must_be_positive CHECK (amount > 0),
    currency    TEXT NOT NULL DEFAULT 'usd',
    customer_id TEXT NOT NULL, 
    description TEXT NOT NULL, 
    created_at  BIGINT NOT NULL,
    updated_at  BIGINT,
    status      ORDER_STATUS NOT NULL DEFAULT 'pending'
);

CREATE TABLE IF NOT EXISTS order_outboxes (
    id             UUID DEFAULT gen_random_uuid() PRIMARY KEY, 
    aggregate_type TEXT NOT NULL, 
    aggregate_id   BIGINT NOT NULL, 
    event_type     TEXT NOT NULL,  
    event_payload  JSONB NOT NULL,
    created_at     BIGINT NOT NULL,
    processed_at   BIGINT,
    status         OUTBOX_STATUS NOT NULL DEFAULT 'pending',

    CONSTRAINT fk_order FOREIGN KEY (aggregate_id) REFERENCES orders(id)
); 

-- TODO: add indexes.
