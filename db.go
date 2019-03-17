package main

import (
	"context"
	"database/sql"
	"log"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

const databaseFile = "exchange.db"

const databaseFullPath = "file:" + databaseFile + "?_sync=3&_txlock=exclusive&_foreign_keys=1"

// the below is from the faq for go-sqlite3, but with the foreign key part added
const databaseTestPath = "file::memory:?mode=memory&cache=shared&_foreign_keys=1"

var ErrNoRows = sql.ErrNoRows

// bunch of crazy stuff in here

var queriesChan = make(chan Query)
var shutdownChan = make(chan struct{})
var shutdownConfirm = make(chan struct{})
var ctx = context.Background()
var lock sync.Mutex

type Query struct {
	exec       func(sql *sql.Tx) error
	completion chan error
}

func SetupDatabase() {
	setupDatabase(databaseFullPath)
}

func SetupDatabaseTestMode() {
	setupDatabase(databaseTestPath)
}

func setupDatabase(fullPath string) {
	lock.Lock()
	log.Println("Opening database file")
	db, err := sql.Open("sqlite3", fullPath)
	if err != nil {
		panic(err)
	}
	log.Println("Database connection created")
	go databaseLoop(db)
	initialSetup()
}

func RunSQL(fn func(sql *sql.Tx) error) error {
	ch := make(chan error)
	queriesChan <- Query{exec: fn, completion: ch}
	return <-ch
}

func databaseLoop(db *sql.DB) {
	defer func() {
		shutdownConfirm <- struct{}{}
	}()
	defer lock.Unlock()
	defer db.Close() // NOTE: this defer is executed FIRST
	for {
		select {
		case <-shutdownChan:
			return
		case query := <-queriesChan:
			query.execute(db)
		}
	}
}

func (q Query) execute(db *sql.DB) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		log.Println("Unable to even begin transaction!")
		q.completion <- err
		return
	}
	err = (q.exec)(tx)
	if err != nil {
		tx.Rollback()
		log.Println("Rolling back database transaction due to error ", err)
		q.completion <- err
		return
	}
	q.completion <- tx.Commit()
}

func ShutdownDatabase() {
	shutdownChan <- struct{}{}
	<-shutdownConfirm
}
