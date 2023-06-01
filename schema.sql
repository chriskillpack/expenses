CREATE TABLE IF NOT EXISTS cursors (
	id INTEGER NOT NULL PRIMARY KEY,
	plaid_item_id TEXT UNIQUE NOT NULL,
	cursor TEXT
);

CREATE TABLE IF NOT EXISTS institutions (
    id INTEGER PRIMARY KEY,
    plaid_institution_id TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    logo BLOB
);

CREATE TABLE IF NOT EXISTS items (
    id INTEGER PRIMARY KEY,
    plaid_access_token TEXT UNIQUE NOT NULL,
    plaid_item_id TEXT UNIQUE NOT NULL,
    plaid_institution_id TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS plaid_transactions (
    id INTEGER PRIMARY KEY,
    plaid_transaction TEXT NOT NULL,
    plaid_transaction_id TEXT UNIQUE NOT NULL
)