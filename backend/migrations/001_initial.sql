CREATE TABLE users (
 id uuid PRIMARY KEY,
 timezone text NOT NULL,
 notification_enabled boolean NOT NULL DEFAULT false,
 suggestion_enabled boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL
);
CREATE TABLE products (
 id uuid PRIMARY KEY,
 sku text NOT NULL UNIQUE CHECK (length(sku) BETWEEN 1 AND 100),
 name text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
 category text NOT NULL,
 brand text NOT NULL,
 unit text NOT NULL,
 replenishable_score double precision NOT NULL CHECK (replenishable_score BETWEEN 0 AND 1),
 available boolean NOT NULL DEFAULT true
);
CREATE TABLE purchases (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id),
 purchased_at timestamptz NOT NULL,
 source text NOT NULL CHECK (source IN ('app','receipt','POS','imported_invoice')),
 total_amount bigint NOT NULL CHECK (total_amount >= 0),
 idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 200),
 payload_hash text NOT NULL,
 UNIQUE(user_id,idempotency_key)
);
CREATE INDEX purchases_user_time ON purchases(user_id,purchased_at);
CREATE TABLE purchase_items (
 purchase_id uuid NOT NULL REFERENCES purchases(id),
 product_id uuid NOT NULL REFERENCES products(id),
 quantity bigint NOT NULL CHECK (quantity>0),
 unit_price bigint NOT NULL CHECK (unit_price>=0),
 PRIMARY KEY(purchase_id,product_id)
);
CREATE INDEX purchase_items_product ON purchase_items(product_id,purchase_id);
CREATE TABLE user_product_state (
 user_id uuid NOT NULL REFERENCES users(id),
 product_id uuid NOT NULL REFERENCES products(id),
 in_cart boolean NOT NULL DEFAULT false,
 in_list boolean NOT NULL DEFAULT false,
 PRIMARY KEY(user_id,product_id)
);
CREATE TABLE decision_logs (
 id uuid PRIMARY KEY,
 evaluation_id uuid NOT NULL,
 user_id uuid NOT NULL REFERENCES users(id),
 product_id uuid NOT NULL REFERENCES products(id),
 engine text NOT NULL,
 shadow boolean NOT NULL,
 decision_context jsonb NOT NULL,
 decision_result jsonb,
 policy_result jsonb,
 executed_action text NOT NULL CHECK (executed_action IN ('DO_NOTHING','WAIT','SUGGEST_NOW','SUGGEST_BUNDLE','ADD_TO_SMART_LIST_SUGGESTION','ASK_IF_RUNNING_LOW')),
 error text,
 latency_ms bigint NOT NULL CHECK (latency_ms>=0),
 created_at timestamptz NOT NULL,
 UNIQUE(evaluation_id,product_id,shadow),
 UNIQUE(id,user_id,product_id),
 CHECK (NOT shadow OR executed_action='DO_NOTHING')
);
CREATE INDEX decisions_user_time ON decision_logs(user_id,created_at DESC,id);
CREATE TABLE suggestions (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id),
 kind text NOT NULL CHECK (kind IN ('SUGGEST_NOW','SUGGEST_BUNDLE','ADD_TO_SMART_LIST_SUGGESTION','ASK_IF_RUNNING_LOW')),
 message text NOT NULL,
 section text NOT NULL CHECK (section IN ('suggestions','suggested_list')),
 status text NOT NULL CHECK (status IN ('pending','accepted','dismissed','fulfilled')),
 created_at timestamptz NOT NULL,
 UNIQUE(id,user_id)
);
CREATE INDEX suggestions_user_time ON suggestions(user_id,created_at DESC,id);
CREATE TABLE suggestion_items (
 suggestion_id uuid NOT NULL,
 user_id uuid NOT NULL,
 product_id uuid NOT NULL REFERENCES products(id),
 decision_id uuid NOT NULL UNIQUE,
 PRIMARY KEY(suggestion_id,product_id),
 FOREIGN KEY(suggestion_id,user_id) REFERENCES suggestions(id,user_id),
 FOREIGN KEY(decision_id,user_id,product_id) REFERENCES decision_logs(id,user_id,product_id),
 UNIQUE(suggestion_id,user_id,product_id,decision_id)
);
CREATE TABLE user_events (
 id uuid PRIMARY KEY,
 user_id uuid NOT NULL REFERENCES users(id),
 event_type text NOT NULL CHECK (event_type IN ('PURCHASED','PRODUCT_VIEWED','ADDED_TO_CART','REMOVED_FROM_CART','ADDED_TO_LIST','REMOVED_FROM_LIST','SUGGESTION_SHOWN','SUGGESTION_ACCEPTED','SUGGESTION_DISMISSED','NOTIFICATION_OPENED')),
 product_id uuid REFERENCES products(id),
 decision_id uuid,
 suggestion_id uuid,
 metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
 created_at timestamptz NOT NULL,
 FOREIGN KEY(decision_id,user_id,product_id) REFERENCES decision_logs(id,user_id,product_id),
 FOREIGN KEY(suggestion_id,user_id,product_id,decision_id) REFERENCES suggestion_items(suggestion_id,user_id,product_id,decision_id),
 CHECK ((decision_id IS NULL AND suggestion_id IS NULL) OR (decision_id IS NOT NULL AND suggestion_id IS NOT NULL AND product_id IS NOT NULL))
);
CREATE INDEX events_user_time ON user_events(user_id,created_at);
CREATE INDEX events_decision ON user_events(decision_id,event_type,created_at);
CREATE UNIQUE INDEX feedback_once ON user_events(decision_id,event_type)
 WHERE event_type IN ('SUGGESTION_SHOWN','SUGGESTION_ACCEPTED','SUGGESTION_DISMISSED');
