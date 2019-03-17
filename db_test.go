package main

import (
	"database/sql"
	"log"
	"testing"
)

// two helper funcs to test the database
func WithTestingDatabase(fn func()) {
	SetupDatabaseTestMode()
	defer ShutdownDatabase()
	fn()
}

func WithTestingSingleQuery(t *testing.T, fn func(sql *sql.Tx) error) {
	WithTestingDatabase(func() {
		err := RunSQL(fn)
		if err != nil {
			t.Error(err)
		}
	})
}

// some base checks that the database works properly
// includes simple queries, commits, rollbacks, and foreign keys
func TestInitialSetup(t *testing.T) {
	WithTestingSingleQuery(t, func(sql *sql.Tx) error {
		var i int64
		err := sql.QueryRow("SELECT 1+1").Scan(&i)
		if err != nil {
			return err
		}
		if i != 2 {
			t.Errorf("1+1 != 2")
		}
		return nil
	})
}

func TestSeparationAndCommit(t *testing.T) {
	WithTestingDatabase(func() {
		err := RunSQL(func(sql *sql.Tx) error {
			_, err := sql.Exec("INSERT INTO users (user_id) VALUES (?)", 5021)
			if err != nil {
				return err
			}

			var user_id int64
			err = sql.QueryRow("SELECT user_id FROM users").Scan(&user_id)
			if err != nil {
				return err
			}
			if user_id != 5021 {
				t.Errorf("Cannot save and fetch within the same transaction")
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var user_id int64
			err := sql.QueryRow("SELECT user_id FROM users").Scan(&user_id)
			if err != nil {
				return err
			}
			if user_id != 5021 {
				t.Errorf("Cannot save and fetch in the same database, but a different transaction")
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})
	WithTestingSingleQuery(t, func(sql *sql.Tx) error {
		var user_id int64
		err := sql.QueryRow("SELECT user_id FROM users").Scan(&user_id)
		if err != ErrNoRows {
			log.Println(user_id)
			t.Errorf("Somehow able to fetch from a different database?")
			t.Error(err)
		}
		return nil
	})
}

func TestRollbackOnError(t *testing.T) {
	WithTestingDatabase(func() {
		err := RunSQL(func(sql *sql.Tx) error {
			_, err := sql.Exec("INSERT INTO users (user_id) VALUES (?)", 5021)
			if err != nil {
				return err
			}
			_, err = sql.Exec("UPDATE users SET balance = balance - 1 WHERE user_id = ?", 5021)
			if err != nil {
				return err
			}
			t.Errorf("Setting a negative balance should have already created a different error!")
			return nil
		})
		if err == nil {
			t.Errorf("Error somehow was not properly returned by db")
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var user_id int64
			err := sql.QueryRow("SELECT user_id FROM users").Scan(&user_id)
			if err != ErrNoRows {
				log.Println(user_id)
				t.Errorf("Somehow able to fetch an insert that should have been rolled back?")
				t.Error(err)
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})
}

func TestForeignKeys(t *testing.T) {
	WithTestingDatabase(func() {
		err := RunSQL(func(sql *sql.Tx) error {
			_, err := sql.Exec("INSERT INTO slots (user_id, slot_index, listing_id) VALUES (?, ?, ?)", 0, 0, 0)
			if err != nil {
				return err
			}
			t.Errorf("That should have returned an error")
			return nil
		})
		if err.Error() != "FOREIGN KEY constraint failed" {
			t.Error(err)
		}
	})
}
