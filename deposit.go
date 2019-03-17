package main

import (
	"database/sql"
	"errors"
	"log"
	"math/rand"
	"strconv"
	"time"
)

const TimeToCompleteDepositSeconds = 15 * 60

func pendingDepositCleanup() {
	ticker := time.NewTicker(time.Second * 10)
	for range ticker.C {
		err := RunSQL(func(sql *sql.Tx) error {
			_, err := sql.Exec("DELETE FROM pending_deposits WHERE expiry_time < strftime('%s', 'now') AND (picked_up_at IS NULL OR picked_up_at < strftime('%s', 'now') - 86400)")
			return err
		})
		if err != nil {
			log.Println("Error while cleaning up unfulfilled pending deposits")
			log.Println(err)
		}
	}
}

// Called when a bot picks up an item and validated it as a correct item to check if the item is a valid deposit
// NOTE: listing_id is server-specific, so this DOES take into account the fact that deposits are different on different servers!
// NOTE 2: The bot ID isn't a parameter as it doesn't matter what bot picks up an item
// NOTE 3: In the case of 2 or more items in the inventory, baritone should only deal with the first one and then consider the others

// Deposit valid -> return true
// Deposit invalid -> return false
func botPicksUpItem(listing_id int64, name uint32) bool {
	err := RunSQL(func(sql *sql.Tx) error {
		var user_id int64
		err := sql.QueryRow("SELECT user_id FROM pending_deposits WHERE deposit_id = ? AND listing_id = ? AND expiry_time > strftime('%s', 'now')", name, listing_id).Scan(&user_id)
		if err != nil {
			return err
		}
		_, err = sql.Exec("UPDATE pending_deposits SET picked_up_at = strftime('%s', 'now') WHERE deposit_id = ?", name)
		if err != nil {
			return err
		}
		go DMuser(user_id, "The deposit bot has picked up your item for listing "+strconv.FormatInt(listing_id, 10)+" and verified its contents and name. Ender chest verification is pending, and should only take a few seconds.")
		return nil // no error
	})
	if err != nil {
		log.Println("Verifying listing", listing_id, "and deposit id", name)
		log.Println("Deposit check error")
		log.Println(err)
		// there was no matching row found in pending_deposits, or there was an error setting picked_up_at
		// either way, it's not safe for the bot to keep this item
		return false
	}
	return true
}

// Called when a bot checks for a deposited item in his enderchest and confirms it
// Specifically, when a bot opens an ender chest, and receives its contents from the server, those contents are 100% verified
// Specifically specifically, when a bot received a SPacketWindowItems or a SPacketSetSlot for a windowid that is a confirmed ender chest, it sends a comms packet
// Which then calls this function

// Everything okay -> return true (keep in echest)
// Something wrong -> return false (bot drops it?)
func confirmedSavedInEchest(bot_uuid string, slot_number int, listing_id int64, name uint32) bool {
	shouldDrop := false
	err := RunSQL(func(sql *sql.Tx) error {
		var user_id int64
		err := sql.QueryRow("SELECT user_id FROM pending_deposits WHERE deposit_id = ? AND listing_id = ?", name, listing_id).Scan(&user_id)
		if err != nil {
			if err != ErrNoRows {
				return err // anything but ErrNoRows is a real error
			}
			// this is NOT a pending deposit
			// therefore we should have it in inventory?
			var expectedOwner string
			var expectedSlot int
			// where should this item be? check using it's item id aka depsosit id aka anvil name
			err = sql.QueryRow("SELECT bot_uuid, slot_number FROM inventory WHERE item_id = ?", name).Scan(&expectedOwner, &expectedSlot)
			if err != nil {
				if err == ErrNoRows {
					shouldDrop = true // it's not a pending deposit and it's not in inventory, this item should be dropped.
				}
				return err
			}
			if expectedOwner != bot_uuid || expectedSlot != slot_number {
				// this is that weird special case I was thinking about
				// if you deposit an identical shulker (name and contents) to two bots at the same time
				// here's the check for it
				return errors.New("This item should be in the care of another bot") // returning an error will make this bot drop this item
				// TODO if they try and trick us like that, should we just keep the item? lol
			}
			return nil // everything is fine, this item is where it's supposed to be, and it's accounted for in the inventory table
		}
		// this was a pending deposit
		_, err = sql.Exec("DELETE FROM pending_deposits WHERE deposit_id = ?", name)
		if err != nil {
			return err
		}
		log.Println("Adding to inventory", listing_id, bot_uuid, slot_number)
		_, err = sql.Exec("INSERT INTO inventory (listing_id, bot_uuid, slot_number, item_id) VALUES (?, ?, ?, ?)", listing_id, bot_uuid, slot_number, name)
		if err != nil {
			return err
		}

		err = fillSlot(sql, user_id, listing_id) // give it to the user
		if err != nil {
			// if they happen to have all slots full and the deposit can't be completed
			// we get here, and return the error below
			// which rolls back this entire transaction, and returns false, which tells the bot to drop the item back to them
			// that's fine and good...
			// but also, in this specific error case only, we redo just the "DELETE FROM pending_deposits WHERE deposit_id = ?"
			// because we don't want to continue in this loop XD
			// i.e. next time we pick it up, we want to reject it immediately, whereas right now it'll keep dropping it and picking it up again lol
			shouldDrop = true
			go clearPending(user_id, name)
			return err
		}

		err = verifyStorage(sql)
		if err != nil {
			return err
		}

		go DMuser(user_id, "Deposit confirmed!!!!!!!!!!")

		return nil
	})
	if err != nil {
		log.Println("Pickup error", err)
	}
	// this system needs to be fail-safe
	// if there is a generic SQL error, we do NOT want to drop all our stock immediately!
	// only if we specifically know that it's missing do we drop
	return !shouldDrop
}

func clearPending(user_id int64, name uint32) {
	err := RunSQL(func(sql *sql.Tx) error {
		// IMPORTANT NOTE 2: by the time we get here, the outer sql transaction is "completed" aka rolled back
		// therefore our above DELETE FROM pending_deposits has been UNDONE
		// therefore we need to REDO it
		_, err := sql.Exec("DELETE FROM pending_deposits WHERE deposit_id = ?", name)
		return err
	})
	if err != nil {
		log.Println("Error while deleting from pending deposits??", err)
	}
	DMuser(user_id, "Error while completing deposit: all of your slots are full. Bot will drop item back to you.")
}

// Creates a pending deposit with a randomly generated ID
func createPendingDeposit(user_id int64, listing_id int64) (int64, error) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	deposit_id := r.Int63()
	err := RunSQL(func(sql *sql.Tx) error {
		_, err := sql.Exec("INSERT INTO pending_deposits (deposit_id, user_id, listing_id, expiry_time) VALUES (?, ?, ?, strftime('%s','now') + ?)", deposit_id, user_id, listing_id, TimeToCompleteDepositSeconds)
		return err
	})
	return deposit_id, err
}
