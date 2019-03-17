package main

import (
	"database/sql"
	"errors"
	"log"
	"math/rand"
	"time"
)

const (
	ForceSellMin = 60 * 5
	ForceSellMax = 60 * 10
)

func slotExpiries() {
	ticker := time.NewTicker(time.Second * 10)
	for range ticker.C {
		checkSlotExpiries() // <^ fancy go trickery. this just schedules checkSlotExpiries to be called every ten seconds lol
	}
}

// pick a random length of time (in seconds) to wait before actually force selling, after expiry
func randomForceSellDelay() int64 {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return int64(r.Intn(ForceSellMax-ForceSellMin+1) + ForceSellMin)
}

func verifyStorage(sql *sql.Tx) error {
	var listing_id int64
	var amountInSlots int
	var amountInEchests int
	err := sql.QueryRow(`
			SELECT debts.listing_id, debts.cnt, storage.cnt FROM
				(SELECT listings.listing_id, COUNT(slots.listing_id)     AS cnt FROM listings LEFT OUTER JOIN slots     ON slots.listing_id = listings.listing_id     GROUP BY listings.listing_id)
			debts INNER JOIN
				(SELECT listings.listing_id, COUNT(inventory.listing_id) AS cnt FROM listings LEFT OUTER JOIN inventory ON inventory.listing_id = listings.listing_id GROUP BY listings.listing_id)
			storage ON debts.listing_id = storage.listing_id WHERE debts.cnt != storage.cnt;
			`).Scan(&listing_id, &amountInSlots, &amountInEchests)
	if err == ErrNoRows {
		return nil // all good here
	}
	return errors.New("UH OH SPAGHETTI O")
}

func checkSlotExpiries() {
	now := time.Now().Unix()
	err := RunSQL(func(sql *sql.Tx) error {
		// auto renew slots that are expired, unlocked, for sale, and have free renewals remaining
		_, err := sql.Exec(`
			UPDATE slots SET
				expiry_time = expiry_time + 86400, /* one day is 86400 seconds */
				renewals = renewals - 1
			WHERE
					expiry_time < ?
				AND
					sale_price IS NOT NULL /* only for sale slots auto renew */
				AND
					renewals > 0
				AND
					locked == 0
			;
				`, now)
		return err
	})
	if err != nil {
		log.Println("Unable to auto renew slots")
		log.Println(err)
		return
	}

	// now that for-sale s have been auto renewed, anything whose expiry is before "now" should be locked and auto sold
	// we use a single consistent variable for now instead of "strftime('%s', 'now')" because what if one second passes between the previous query and this one?

	err = RunSQL(func(sql *sql.Tx) error {

		row := sql.QueryRow("SELECT user_id, slot_index, listing_id FROM slots WHERE locked == 0 AND expiry_time < ? ORDER BY expiry_time ASC LIMIT 1", now)
		var user_id int64
		var slot_index int
		var listing_id int64
		err := row.Scan(&user_id, &slot_index, &listing_id)
		if err != nil {
			if err == ErrNoRows {
				// there are no slots to lock for force selling. oh well.
				// we ignore this error and return nil because this isn't truly an error, there just weren't any rows given back from that select statement
				return nil
			} else {
				return err
			}
		}
		// we found one slot, there might be more
		// check in another thread
		go checkSlotExpiries()
		// back to this one...
		delay := randomForceSellDelay()
		_, err = sql.Exec("UPDATE slots SET locked = 1, sale_price = NULL, for_sale_since = NULL, expiry_time = ? WHERE user_id = ? AND slot_index = ?", now+delay, user_id, slot_index)
		log.Println("Saying it'll take between 5 and 10 minutes but it'll really be force sold in exactly", delay, "seconds")

		log.Println("Here I would notify discord that 1 shulker of", listing_id, "will be auto force sold sometime in the next 5 to 10 minutes. Put in buy orders if you want it, it goes to the highest open one!")
		return err
	})
	if err != nil {
		log.Println("Unable to expire slots")
		log.Println(err)
		return
	}

	err = RunSQL(func(sql *sql.Tx) error {
		row := sql.QueryRow("SELECT user_id, slot_index, listing_id FROM slots WHERE locked == 1 AND expiry_time < ? AND (sale_price IS NULL OR sale_price > 0) ORDER BY expiry_time ASC LIMIT 1", now)
		// grab all rows that are locked and expired, where they're not for sale, or for sale for a price that's greater than zero
		// this prevents us from force selling the same item over and over, it's okay to leave the order up for 0 each, indefinitely
		var user_id int64
		var slot_index int
		var listing_id int64
		err := row.Scan(&user_id, &slot_index, &listing_id)
		if err != nil {
			if err == ErrNoRows {
				// there are no slots to force sell. oh well.
				// we ignore this error and return nil because this isn't truly an error, there just weren't any rows given back from that select statement
				return nil
			} else {
				return err
			}
		}

		// you can't sell against yourself
		// so before putting the sell order up for free, we cancel any buy orders they may have had up in this listing
		err = cancelAllBuysInListing(sql, user_id, listing_id)

		if err != nil {
			return err
		}

		err = createSellOrder(sql, user_id, slot_index, 0)

		if err != nil {
			return err
		}

		log.Println("Sorry dude, it's force selling")
		log.Println("Here I would notify discord on the same channel that the force sell order has been placed")
		return nil
	})
	if err != nil {
		log.Println("Unable to force sell slots")
		log.Println(err)
		return
	}
}
