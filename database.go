package main

import (
	"fmt"
	"log"
	"math/big"

	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	Interface *sql.DB
}

func DbOpen(config *Config) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@%s(%s)/%s",
		config.DBUser,
		config.DBPass,
		config.DBProtocol,
		config.DBHostname,
		config.DBName,
	)

	log.Printf("Connecting to DB using dsn: %s", dsn)

	dbInterface, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	dbInterface.SetMaxIdleConns(100)

	err = dbInterface.Ping()
	if err != nil {
		return nil, err
	}

	log.Println("Connected to DB")

	db := new(DB)
	db.Interface = dbInterface

	return db, nil
}

func (db *DB) Close() {
	if db.Interface != nil {
		db.Interface.Close()
		db.Interface = nil
	}
}

func (db *DB) InitTables() error {
	queries := []string{`
		CREATE TABLE eth_keys(
			id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			address VARCHAR(40),
			private VARCHAR(64)
		);`,
		`CREATE INDEX eth_keys_address_idx ON eth_keys(address);`,
		`CREATE TABLE notifications(
			id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			address_from     VARCHAR(40),
			address_to       VARCHAR(40),
			address_contract VARCHAR(40),
			amount           VARCHAR(32),
			is_pending       BOOLEAN NOT NULL DEFAULT false,
			tx_hash          VARCHAR(64),
			created_at       DATETIME DEFAULT NOW()
		);`,
		`CREATE TABLE settings(
			id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(32) UNIQUE,
			value VARCHAR(64)
		);`,
	}

	for _, query := range queries {
		_, err := db.Interface.Query(query)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) InsertKey(address, private string) error {
	stmt, err := db.Interface.Prepare("INSERT INTO eth_keys(address, private) VALUES(?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(address, private)
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) InsertNotification(
	address_from, address_to, address_contract, amount string,
	is_pending bool, tx_hash string) error {

	stmt, err := db.Interface.Prepare(`
		INSERT INTO notifications(address_from, address_to, address_contract, amount, is_pending, tx_hash)
		VALUES(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(address_from, address_to, address_contract, amount, is_pending, tx_hash)
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) IsAddressKnown(address string) (bool, error) {
	stmt, err := db.Interface.Prepare("SELECT id FROM eth_keys WHERE address = LOWER(?)")
	if err != nil {
		return false, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(address)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if false == rows.Next() {
		return false, nil
	}

	return true, nil
}

func (db *DB) GetSetting(name string) (string, error) {
	var value string

	stmt, err := db.Interface.Prepare("SELECT value FROM settings WHERE name = LOWER(?)")
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	err = stmt.QueryRow(name).Scan(&value)
	if err != nil {
		return "", err
	}

	return value, nil
}

func (db *DB) SetSetting(name, value string) error {
	stmt, err := db.Interface.Prepare(`INSERT INTO settings(name, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(name, value, value)
	if err != nil {
		return err
	}

	return nil
}

func (db *DB) GetNotifications(remove bool) ([]NotifyMessage, error) {
	var id uint64

	stmt, err := db.Interface.Prepare(
		`SELECT id, address_from, address_to, address_contract, amount, is_pending, tx_hash
		 FROM notifications ORDER BY id ASC LIMIT 100`)
	if err != nil {
		return []NotifyMessage{}, err
	}
	defer stmt.Close()

	msgs := make([]NotifyMessage, 0)

	rows, err := stmt.Query()
	for rows.Next() {
		var msg NotifyMessage
		var amount uint64

		err := rows.Scan(
			&id,
			&msg.AddressFrom,
			&msg.AddressTo,
			&msg.ContractAddress,
			&amount,
			&msg.IsPending,
			&msg.TxHash,
		)

		msg.Amount = new(big.Int)
		msg.Amount.SetUint64(amount)

		if err != nil {
			return []NotifyMessage{}, err
		}

		msgs = append(msgs, msg)
	}

	if err := rows.Err(); err != nil {
		return []NotifyMessage{}, err
	}

	if false == remove {
		return msgs, nil
	}

	// Remove notifications from database
	stmt, err = db.Interface.Prepare("DELETE FROM notifications WHERE id <= ?")
	if err != nil {
		return msgs, err
	}

	_, err = stmt.Exec(id)
	if err != nil {
		return msgs, err
	}

	return msgs, nil
}
