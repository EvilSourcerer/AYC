package main

import (
	"database/sql"
	"errors"
	"log"
	"strconv"
)

// can only be called within the context of a sql transaction
// this is intentional, to preserve atomicity
func createSellOrder(sql *sql.Tx, user_id int64, slot_index int, price int) error {
	if price < 0 {
		return errors.New("Cannot sell for less than zero each")
	}
	row := sql.QueryRow("SELECT locked, listing_id FROM slots WHERE user_id = ? AND slot_index = ?", user_id, slot_index)
	var locked int
	var listing_id int64
	err := row.Scan(&locked, &listing_id)
	if err != nil {
		return err // it IS an error if the select does not find a slot for this user and index
	}
	if locked == 2 {
		return errors.New("Cannot put something up for sale that you're in the middle of withdrawing")
	}
	if locked == 1 && price != 0 {
		return errors.New("Slot is locked for force selling, cannot put up for sale for nonzero price")
	}

	// let's grab the highest buy order that we could execute against immediately
	// this is any buy order that is for this listing whose buy price is greater than or equal to the sell price
	row = sql.QueryRow("SELECT user_id, quantity, price FROM listing_buy_orders WHERE listing_id = ? AND price >= ? ORDER BY price DESC, created_at ASC LIMIT 1", listing_id, price)
	var buyer_id int64
	var buy_quantity int
	var buy_price int64
	err = row.Scan(&buyer_id, &buy_quantity, &buy_price)
	if err != nil {
		if err == ErrNoRows {
			// there is no buyer
			// therefore we can just put this up for sale without having to worry about that
			_, err = sql.Exec("UPDATE slots SET sale_price = ?, for_sale_since = strftime('%s', 'now') WHERE user_id = ? AND slot_index = ?", price, user_id, slot_index)
			// all done
		}
		return err
	}

	if buyer_id == user_id {
		return errors.New("Cannot place a sell order that would match against one of your own buy orders")
	}

	// note that buy_price >= price, as guaranteed by the above select

	// this trade is going to go through at buy_price, by the rule of the first order gets the price
	tradePrice := buy_price
	// this means that the seller could possibly get MORE rc than expected from this sale
	// while the buyer will get exactly what their buy order was for

	// this buy order was fulfilled
	// we need to decrease the quantity of the order by one
	// because sql is stupid, we need to delete it if it's 1, then try to decement it otherwise
	_, err = sql.Exec("DELETE FROM listing_buy_orders WHERE quantity = 1       AND user_id = ? AND listing_id = ? AND price = ?", buyer_id, listing_id, buy_price)
	if err != nil {
		return err
	}
	_, err = sql.Exec("UPDATE listing_buy_orders SET quantity = quantity - 1 WHERE user_id = ? AND listing_id = ? AND price = ?", buyer_id, listing_id, buy_price)
	if err != nil {
		return err
	}

	return executeTrade(sql, user_id, slot_index, buyer_id, listing_id, tradePrice)
}

func createBuyOrder(sql *sql.Tx, user_id int64, listing_id int64, price int, quantity int) error {
	if price <= 0 {
		return errors.New("Cannot create buy for 0 or less each")
	}
	if quantity <= 0 {
		return errors.New("Cannot create buy for quantity of 0 or less")
	}

	var currentFullSlots int
	err := sql.QueryRow("SELECT COUNT(*) FROM slots WHERE user_id = ?", user_id).Scan(&currentFullSlots)
	if err != nil {
		return err
	}
	log.Println("Currently full", currentFullSlots)
	var maxSlots int
	var balance int64
	err = sql.QueryRow("SELECT max_slots, balance FROM users WHERE user_id = ?", user_id).Scan(&maxSlots, &balance)
	if err != nil {
		return err
	}

	if currentFullSlots >= maxSlots {
		return errors.New("Cannot place a buy order because there are no open slots the purchased item could go in")
	}

	// blehhh.... this is safe because price and quantity are ints and are at most 2^31-1... https://www.wolframalpha.com/input/?i=((2%5E31-1)%5E2)+%2F+(2%5E64-1) it's okay
	cost := int64(price) * int64(quantity)

	if cost > balance || cost < 0 {
		return errors.New("Cannot place a buy order for more " + Currency + " than you have")
	}

	var executedCost int64

	for quantity > 0 {
		// let's try matching one against an existing order
		row := sql.QueryRow(`SELECT user_id, slot_index, sale_price FROM slots

			WHERE
				listing_id = ?
			AND
				sale_price IS NOT NULL
			AND
				sale_price <= ?

			ORDER BY sale_price ASC, for_sale_since ASC /* this orders by sale_price, with the tiebreaker being for_sale_since */

			LIMIT 1`, listing_id, price)

		var seller_id int64
		var seller_slot_index int
		var sale_price int64
		err = row.Scan(&seller_id, &seller_slot_index, &sale_price)
		if err != nil {
			if err == ErrNoRows {
				// there is no seller that can be matched immediately
				break
			}
			return err
		}
		// there is a seller!

		executedCost += sale_price

		if seller_id == user_id {
			return errors.New("Cannot place a buy order that would match against one of your own sell orders")
		}

		quantity--
		// trade prace is sale price because it's a better deal for the buyer, and follows the first-order rule
		err = executeTrade(sql, seller_id, seller_slot_index, user_id, listing_id, sale_price)
		if err != nil {
			return err
		}

		currentFullSlots++

		if currentFullSlots >= maxSlots {
			// we were able to partially fulfill this order
			// this can happen when you have 1 open slot, but you put in a buy order for a quantity of 2
			// this is allowed
			// however, it cancels all other buy orders, and the rest of this one
			err = cancelAllBuys(sql, user_id) // may have already been called by executeTrade, but just be sure.
			if err != nil {
				return err
			}
			// they still have to pay for what they bought so far

			_, err = sql.Exec("UPDATE users SET balance = balance - ? WHERE user_id = ?", executedCost, user_id)
			return err
		}
	}

	// quantity has been decerement to just remaining quantity
	newBalance := balance - int64(price)*int64(quantity) - executedCost

	// even if quantity is now zero, fully matched, they still have to pay
	_, err = sql.Exec("UPDATE users SET balance = ? WHERE user_id = ?", newBalance, user_id)
	if err != nil {
		return err
	}

	if quantity == 0 {
		// we were able to match all of this buy order against existing sell orders
		return nil // no error
	}

	// the remaining quantity is an open buy offer
	_, err = sql.Exec("INSERT INTO listing_buy_orders (user_id, listing_id, quantity, price) VALUES (?, ?, ?, ?)", user_id, listing_id, quantity, price)
	if err != nil {
		// already have one here
		_, err = sql.Exec("UPDATE listing_buy_orders SET quantity = quantity + ? WHERE user_id = ? AND listing_id = ? AND price = ?", quantity, user_id, listing_id, price)
		return err
	}
	// all done
	return nil // no error
}

func executeTrade(sql *sql.Tx, seller_id int64, seller_slot_index int, buyer_id int64, listing_id int64, tradePrice int64) error {
	log.Println("1 item from listing", listing_id, "is being sold by", seller_id, "to", buyer_id, "for", tradePrice)

	// buy order is decremented, now to transfer the RC
	// note that as part of placing a buy order, your RC is locked up in the order
	// therefore we DON'T decrease the balance of the buyer, we only increase the balance of the seller
	// decrementing the size of their order is effectively what takes the money from the buyer
	_, err := sql.Exec("UPDATE users SET balance = balance + ? WHERE user_id = ?", tradePrice, seller_id)
	if err != nil {
		return err
	}

	// remove it from the seller
	_, err = sql.Exec("DELETE FROM slots WHERE user_id = ? AND slot_index = ?", seller_id, seller_slot_index)
	if err != nil {
		return err
	}

	// give it to the buyer
	err = fillSlot(sql, buyer_id, listing_id)
	if err != nil {
		return err
	}

	err = verifyStorage(sql)
	if err != nil {
		return err
	}

	_, err = sql.Exec("INSERT INTO completed_listing_trades (buyer_id, seller_id, listing_id, price) VALUES (?, ?, ?, ?)", buyer_id, seller_id, listing_id, tradePrice)
	if err != nil {
		return err
	}

	// TODO this should only send out once it's guaranteed to have happened
	go DMuser(seller_id, "You just sold an item! Balance increased by "+strconv.FormatInt(tradePrice, 10)+Currency+".")
	go DMuser(buyer_id, "You just bought an item!")
	return nil
	// note that by the magic of sql transactions, every previous query will get automatically rolled back as if they never happened, if this ends up returning any error
}

// give a user an item
func fillSlot(sql *sql.Tx, user_id int64, listing_id int64) error {
	// how many slots do they have?
	row := sql.QueryRow("SELECT max_slots FROM users WHERE user_id = ?", user_id)
	var slotCount int
	err := row.Scan(&slotCount)
	if err != nil {
		return err
	}

	for i := 0; i < slotCount; i++ {
		// try giving them the item in slot i
		_, err = sql.Exec("INSERT INTO slots (user_id, slot_index, listing_id) VALUES (?, ?, ?)", user_id, i, listing_id) // leave all other columns at defaults
		if err != nil {
			log.Println("Unable to give this item to them in slot", i, "because of insertion error", err)
			// most likely, they already have something in that slot
			// and the error is from the constraint UNIQUE(user_id, slot_index),
			continue
		}
		log.Println("Completed trade. Item is now in slot", i, "of their account")

		var currentFullSlots int
		err = sql.QueryRow("SELECT COUNT(*) FROM slots WHERE user_id = ?", user_id).Scan(&currentFullSlots)
		if err != nil {
			return err
		}
		if currentFullSlots >= slotCount {
			log.Println("This filled up their last slot. Time to cancel all of their open buy orders, because they can't be fulfilled.")
			err = cancelAllBuys(sql, user_id)
			if err != nil {
				return err
			}
		}

		return nil // we're done, no error
	}
	// note that this shouldn't be possible, because you can only place a buy order if you have slots available
	return errors.New("No available slots to place the item into")
}

func cancelAllBuys(sql *sql.Tx, user_id int64) error {
	var totalRefund int64
	err := sql.QueryRow("SELECT COALESCE(SUM(price * quantity), 0) FROM listing_buy_orders WHERE user_id = ?", user_id).Scan(&totalRefund)
	if err != nil {
		return err
	}

	_, err = sql.Exec("UPDATE users SET balance = balance + ? WHERE user_id = ?", totalRefund, user_id)
	if err != nil {
		return err
	}

	_, err = sql.Exec("DELETE FROM listing_buy_orders WHERE user_id = ?", user_id)
	return err
}

func cancelAllBuysInListing(sql *sql.Tx, user_id int64, listing_id int64) error {
	var totalRefund int64
	err := sql.QueryRow("SELECT COALESCE(SUM(price * quantity), 0) FROM listing_buy_orders WHERE user_id = ? AND listing_id = ?", user_id, listing_id).Scan(&totalRefund)
	if err != nil {
		return err
	}

	_, err = sql.Exec("UPDATE users SET balance = balance + ? WHERE user_id = ?", totalRefund, user_id)
	if err != nil {
		return err
	}

	_, err = sql.Exec("DELETE FROM listing_buy_orders WHERE user_id = ? AND listing_id = ?", user_id, listing_id)
	return err
}

func cancelSpecificBuy(user_id int64, listing_id int64, price int) error {
	return RunSQL(func(sql *sql.Tx) error {
		var totalRefund int64
		err := sql.QueryRow("SELECT COALESCE(SUM(price * quantity), 0) FROM listing_buy_orders WHERE user_id = ? AND listing_id = ? AND price = ?", user_id, listing_id, price).Scan(&totalRefund)
		if err != nil {
			return err
		}

		_, err = sql.Exec("UPDATE users SET balance = balance + ? WHERE user_id = ?", totalRefund, user_id)
		if err != nil {
			return err
		}

		_, err = sql.Exec("DELETE FROM listing_buy_orders WHERE user_id = ? AND listing_id = ? AND price = ?", user_id, listing_id, price)
		return err
	})
}

func cancelSell(user_id int64, slot_index int) error {
	return RunSQL(func(sql *sql.Tx) error {
		// no need to check locked, withdrawal_code, etc, database constraints will take care of that
		_, err := sql.Exec("UPDATE slots SET sale_price = NULL, for_sale_since = NULL WHERE user_id = ? AND slot_index = ?", user_id, slot_index)
		return err
	})
}
