package expenses

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"strconv"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"github.com/plaid/plaid-go/v12/plaid"
)

//go:embed schema.sql
var sqlSchema string

type DB struct {
	mu sync.Mutex
	db *sql.DB
}

type Item struct {
	Id                 int
	AccessToken        string
	PlaidItemId        string
	PlaidInstitutionId string
}

type Institution struct {
	Id                 int
	PlaidInstitutionId string
	Name               string
	Logo               string // base64 encoded PNG
}

func (db *DB) Close() {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.db.Close()
}

func NewDB(fname string) (*DB, error) {
	db, err := sql.Open("sqlite3", fname)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}
	if _, err := db.Exec(sqlSchema); err != nil {
		return nil, err
	}

	return &DB{db: db}, nil
}

func (db *DB) CreateNewItem(ctx context.Context, id, access_token, institution_id string) error {
	_, err := db.db.ExecContext(
		ctx,
		`INSERT INTO items
			(plaid_item_id,plaid_access_token,plaid_institution_id)
		VALUES ($1, $2, $3);`, id, access_token, institution_id)

	return err
}

func (db *DB) RetrieveItems(ctx context.Context) ([]*Item, error) {
	rows, err := db.db.QueryContext(
		ctx,
		`SELECT id,
				plaid_access_token,
				plaid_item_id,
				plaid_institution_id
		 FROM items`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanItems(rows)
}

func (db *DB) RetrieveItemsByPlaidInstitutionId(ctx context.Context, institutionId string) ([]*Item, error) {
	rows, err := db.db.QueryContext(
		ctx,
		`SELECT id,
				plaid_access_token,
				plaid_item_id,
				plaid_institution_id
		 FROM items WHERE plaid_institution_id=$1`, institutionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanItems(rows)
}

func (db *DB) UniqueInstitutionIds(ctx context.Context) ([]string, error) {
	rows, err := db.db.QueryContext(
		ctx,
		`SELECT DISTINCT plaid_institution_id
		 FROM items`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var institutions []string
	for rows.Next() {
		var id string
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		institutions = append(institutions, id)
	}

	return institutions, nil
}

func (db *DB) RetrieveInstitutionById(ctx context.Context, institutionId string) (*Institution, error) {
	row := db.db.QueryRowContext(
		ctx,
		`SELECT id,
			    plaid_institution_id,
				name,
				logo
		 FROM institutions
		 WHERE plaid_institution_id=$1`, institutionId)

	institution := &Institution{}
	err := row.Scan(&institution.Id, &institution.PlaidInstitutionId, &institution.Name, &institution.Logo)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}

		return nil, err
	}

	return institution, err
}

func (db *DB) UpdateInstitution(ctx context.Context, institution *Institution) error {
	if institution.Id == 0 {
		// It's a new record, insert it
		_, err := db.db.ExecContext(
			ctx,
			`INSERT INTO institutions
				(plaid_institution_id,name,logo)
			 VALUES ($1, $2, $3)`, institution.PlaidInstitutionId, institution.Name, institution.Logo)
		return err
	} else {
		// Update an existing record
		_, err := db.db.ExecContext(
			ctx,
			`UPDATE institutions
				SET plaid_institution_id=$1, name=$2, logo=$3
			WHERE id=$4`, institution.PlaidInstitutionId, institution.Name, institution.Logo, institution.Id)
		return err
	}
}

func (db *DB) GetItemCursorOrNil(ctx context.Context, item_id string) (string, error) {
	row := db.db.QueryRowContext(
		ctx,
		`SELECT cursor
		 FROM cursors
		 WHERE plaid_item_id=$1`, item_id)
	var cursor string
	err := row.Scan(&cursor)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	return cursor, nil
}

func (db *DB) UpdatePlaidTransactions(ctx context.Context, added []plaid.Transaction, removed []plaid.RemovedTransaction, plaid_item_id, cursor string) (int64, error) {
	// Insert into the DB in batches of 5
	const batchSize = 5

	txn, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, nil
	}
	defer txn.Rollback()

	start := 0
	var affected int64
	for start < len(added) {
		end := start + batchSize
		if end > len(added) {
			end = len(added)
		}

		queryString := "INSERT INTO plaid_transactions (plaid_transaction, plaid_transaction_id, deleted) VALUES"
		params := make([]any, batchSize*2)
		for idx, item := range added[start:end] {
			queryString = queryString + " ($" + strconv.Itoa(idx*2+1) + ",$" + strconv.Itoa(idx*2+2) + ",0),"

			jsontxn, err := json.Marshal(item)
			if err != nil {
				return 0, err
			}

			params[idx*2+0] = string(jsontxn)
			params[idx*2+1] = item.GetTransactionId()
		}
		// Remove the trailing colon
		queryString = queryString[0 : len(queryString)-1]

		res, err := txn.Exec(queryString, params...)
		if err != nil {
			return 0, err
		}
		ra, err := res.RowsAffected()
		if err != nil {
			return 0, err
		}

		affected += ra

		start = end
	}

	// Go through and mark any removed transactions
	for _, rem := range removed {
		txnid := rem.GetTransactionId()

		_, err := txn.Exec("UPDATE plaid_transactions SET deleted=1 WHERE plaid_transaction_id=$1", txnid)
		if err != nil {
			return 0, err
		}
	}

	// Finally update the cursor
	_, err = txn.Exec(`
		REPLACE INTO cursors (plaid_item_id, cursor)
		VALUES ($1, $2)`, plaid_item_id, cursor)
	if err != nil {
		return 0, err
	}

	if err = txn.Commit(); err != nil {
		return 0, err
	}

	return affected, nil
}

func scanItems(rows *sql.Rows) ([]*Item, error) {
	var items []*Item
	for rows.Next() {
		item := new(Item)
		err := rows.Scan(&item.Id, &item.AccessToken, &item.PlaidItemId, &item.PlaidInstitutionId)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}
