package main

import (
	"database/sql"
	"errors"
	"log"
	"math/rand"
	"strconv"
	"time"
)

const TimeToCompleteWithdrawalSeconds = 15 * 60

func pendingWithdrawalCleanup() {
	ticker := time.NewTicker(time.Second * 10)
	for range ticker.C {
		err := RunSQL(func(sql *sql.Tx) error {
			row := sql.QueryRow("SELECT withdrawal_code FROM pending_withdrawals WHERE expiry_time < strftime('%s', 'now') LIMIT 1")
			var withdrawal_code int64
			err := row.Scan(&withdrawal_code)
			if err != nil {
				if err == ErrNoRows {
					return nil
				}
				return err
			}
			_, err = sql.Exec(`UPDATE slots SET withdrawal_code = NULL, locked = 0 WHERE withdrawal_code = ?;
				               DELETE FROM pending_withdrawals WHERE withdrawal_code = ?`, withdrawal_code, withdrawal_code)
			return err
		})
		if err != nil {
			log.Println("Error while cleaning up unfulfilled pending deposits")
			log.Println(err)
		}
	}
}

// when a bot receives a message like "2b2tq.org withdrawal #abcd1234" through ANY means (/w or normal chat)
// it will call this function
// oh also there should be a public API thing like /withdraw?code=abcd1234 that just hex decodes then calls this
// that way you can have an easy button on the site to do it
// this is the final step in completing a withdrawal (first you get the code and bot XYZ, then you go there and message it the code to make it actually drop)
func botReceivesWithdrawalCode(code int64) {
	err := RunSQL(func(sql *sql.Tx) error {
		// quad table join like a sir *nae nae*
		row := sql.QueryRow("SELECT slots.user_id, slots.slot_index, pending_withdrawals.item_id, inventory.bot_uuid, inventory.slot_number, listings.item_name, listings.server FROM slots INNER JOIN pending_withdrawals ON pending_withdrawals.withdrawal_code = slots.withdrawal_code INNER JOIN inventory ON inventory.item_id = pending_withdrawals.item_id INNER JOIN listings ON listings.listing_id = inventory.item_id WHERE slots.withdrawal_code = ?", code)
		var user_id int64
		var slot_index int // the slot index that is being withdrawn from (slot on the website)
		var item_id uint32
		var bot_uuid string
		var slot_number int // the slot number that is being dropped (slot in an ender chest)
		var item_name string
		var server string
		err := row.Scan(&user_id, &slot_index, &item_id, &bot_uuid, &slot_number, &item_name, &server)
		if err != nil {
			// no such withdrawal
			return err
		}

		_, err = sql.Exec(`DELETE FROM slots               WHERE withdrawal_code = ?;
			               DELETE FROM pending_withdrawals WHERE withdrawal_code = ?;
			               DELETE FROM inventory           WHERE item_id         = ?;`, code, code, item_id)

		if err != nil {
			return err
		}

		err = verifyStorage(sql) // always sanity check after modifying inventory or slots, and rollback on failure
		if err != nil {
			return err
		}

		notificationMessage := "Withdrawal code `" + strconv.FormatInt(code, 10) + "` confirmed!\n"
		notificationMessage += "The shulker of `" + item_name + "` on `" + server + "` will be dropped, and has been removed from slot `#" + strconv.Itoa(slot_index) + "` of your exchange account.\n"
		notificationMessage += "The item name will be `" + depositIDToName(item_id) + "`.\n\n"
		notificationMessage += "UUID of the bot that has this item in its ender chest is `" + bot_uuid + "`.\n"
		bot := getByUUIDAndServer(bot_uuid, server)
		if bot == nil || bot.latestStatus == nil || bot.latestStatus.Dimension != 0 || !bot.hasReceivedStatusUpdateInTheLastFiveSeconds() {
			notificationMessage += "This bot is not currently online and connected to the exchange controller, which is strange because you should not have been able to place this withdrawal in the first place.\n"
			notificationMessage += "It will drop the item soon as it regains connection.\n"
		} else {
			status := bot.latestStatus
			notificationMessage += "This bot is at (" + strconv.Itoa(int(status.X)) + "," + strconv.Itoa(int(status.Y)) + "," + strconv.Itoa(int(status.Z)) + ") and will drop your item immediately.\n"
		}
		go DMuser(user_id, notificationMessage)
		return nil
	})
	if err != nil {
		log.Println("Unable to complete withdrawal", err)
	}
}

func createWithdrawal(user_id int64, slot_index int) (int64, error) {
	var withdrawal_code int64
	err := RunSQL(func(sql *sql.Tx) error {
		row := sql.QueryRow("SELECT slots.listing_id, slots.locked, listings.server FROM slots INNER JOIN listings ON listings.listing_id = slots.listing_id WHERE slots.user_id = ? AND slots.slot_index = ? AND slots.locked = 0 AND slots.expiry_time > strftime('%s','now')", user_id, slot_index)
		var listing_id int64
		var locked int64
		var server string
		err := row.Scan(&listing_id, &locked, &server)
		if err != nil {
			return err // cannot withdraw something that you don't have, something that's expired, something that's already up for withdrawal, etc
		}
		if locked == 1 {
			return errors.New("cannot withdraw locked meme")
		}
		currentlyConnected := getConnectedToServer(server)
		uuids := make(map[string]bool)
		for _, bot := range currentlyConnected {
			if !bot.hasReceivedStatusUpdateInTheLastFiveSeconds() {
				continue
			}
			if bot.latestStatus.Dimension != 0 {
				continue
			}
			uuids[bot.latestStatus.BotUUID] = true
		}
		item_id, err := getMeAWithdrawalOption(sql, listing_id, uuids)
		if err != nil {
			return err
		}
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		withdrawal_code = r.Int63()
		_, err = sql.Exec("INSERT INTO pending_withdrawals (withdrawal_code, item_id, expiry_time) VALUES (?, ?, strftime('%s','now') + ?)", withdrawal_code, item_id, TimeToCompleteWithdrawalSeconds)
		if err != nil {
			return err
		}
		_, err = sql.Exec("UPDATE slots SET withdrawal_code = ?, locked = 2, sale_price = NULL, for_sale_since = NULL WHERE user_id = ? AND slot_index = ?", withdrawal_code, user_id, slot_index)
		if err != nil {
			return err
		}
		return nil
	})
	return withdrawal_code, err
}

func getMeAWithdrawalOption(sql *sql.Tx, listing_id int64, uuids map[string]bool) (int64, error) {
	// select rows from inventory that are not currently pending withdrawals
	rows, err := sql.Query("SELECT inventory.item_id, inventory.bot_uuid FROM inventory LEFT OUTER JOIN pending_withdrawals ON pending_withdrawals.item_id = inventory.item_id WHERE pending_withdrawals.withdrawal_code IS NULL AND inventory.listing_id = ?", listing_id)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var item_id int64
		var bot_uuid string
		err = rows.Scan(&item_id, &bot_uuid)
		if err != nil {
			return 0, err
		}
		_, ok := uuids[bot_uuid]
		if ok {
			return item_id, nil
		}
	}
	err = rows.Err()
	if err != nil {
		return 0, err
	}
	return 0, errors.New("Unable to schedule your withdrawal; none of the bots with that item are currently connected. Please wait 30 minutes and try again.")
}
