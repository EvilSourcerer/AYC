package main

import (
	"database/sql"
	"log"
)

func initialSetup() {
	err := RunSQL(func(sql *sql.Tx) error {
		_, err := sql.Exec(`CREATE TABLE IF NOT EXISTS users (

			user_id    INTEGER NOT NULL PRIMARY KEY,                     /* use discord id not @# tag because you can change your username */
			balance    INTEGER NOT NULL DEFAULT 0,                       /* this verifies and guarantees at the database level that your balance can never be negative */
			max_slots  INTEGER NOT NULL DEFAULT 4,                       /* how many slots this user has */
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), /* when this account was created */

			CHECK(balance >= 0),
			CHECK(max_slots >= 0),
			CHECK(user_id > 0),
			CHECK(created_at > 0)

		);`)
		if err != nil {
			log.Println("Unable to create users table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS listings (

			listing_id INTEGER NOT NULL PRIMARY KEY, /* id of this listing */
			item_key   TEXT    NOT NULL,             /* e.g. enchanted golden apple is "item.appleGold;1", it's translation key semicolon damage value */
			server     TEXT    NOT NULL,             /* what server this is on, 2b2t or whatever */
			item_name  TEXT    NOT NULL,             /* name of the item, like "shulker of gapples" */
			item_photo TEXT    NOT NULL,             /* i guess the URL of the image of this? like "/static/gapple.png" */

			UNIQUE(server, item_key), /* can't have two listings for the same item on the same server */
			CHECK(LENGTH(item_key) > 0 AND LENGTH(server) > 0 AND LENGTH(item_name) > 0 AND LENGTH(item_photo) > 0),
			CHECK(listing_id >= 0) /* makes the code easier if they never start with a -. shrug. */
		);
		CREATE INDEX IF NOT EXISTS listingsserveritem ON listings(server, item_key);`)
		if err != nil {
			log.Println("Unable to create listings table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS slots (  /* the big one */

			user_id         INTEGER NOT NULL,                                         /* which user owns this slot */
			slot_index      INTEGER NOT NULL,                                         /* which slot this is for the user that owns it, 0 through 3 */
			listing_id      INTEGER NOT NULL,                                         /* what listing is in this slot, empty slots have no row in this table */
			expiry_time     INTEGER NOT NULL DEFAULT (strftime('%s', 'now') + 86400), /* when will this slot expire, unix time, seconds since epoch */
			renewals        INTEGER NOT NULL DEFAULT 14,                              /* how many more times can this slot be renewed */
			sale_price      INTEGER,                                                  /* how much it's for sale for, NULL means not for sale. note that 0 DOES mean for sale, just up for grabs for free. force sells are at price 0. */
			for_sale_since  INTEGER,                                                  /* when it was put up for sale */
			locked          INTEGER NOT NULL DEFAULT 0,                               /* 0: not locked, 1: locked for force sale, 2: withdrawal in progress */
			withdrawal_code INTEGER,                                                  /* reference to pending_withdrawals table, NULL if not currently in process of withdrawal */

			UNIQUE(user_id, slot_index),                                                                          /* a user can't have two different items in the same slot */
			CHECK(slot_index >= 0),
			CHECK(expiry_time > 0),
			CHECK(renewals >= 0),
			CHECK(sale_price IS NULL OR sale_price >= 0),                                                         /* sale price can never be negative but it can be null */
			CHECK(locked == 0 OR locked == 1 OR locked == 2),                                                     /* locked is basically an enum, must be 0 through 2 */
			CHECK(sale_price IS NULL OR locked != 2),                                                             /* AKA something cannot be for sale and in the middle of withdrawal AKA it cannot be true that: sale_price IS NOT NULL AND locked == 2 */
			CHECK(sale_price IS NULL OR sale_price == 0 OR locked != 1),                                          /* AKA something cannot be locked for force sale and for sale for a nonzero price AKA it cannot be true that: sale_price IS NOT NULL AND sale_price > 0 AND locked == 1 */
			CHECK((withdrawal_code IS NULL AND locked != 2) OR (withdrawal_code IS NOT NULL AND locked == 2)),                 /* make sure withdrawal_code and locked==2 are in sync */
			CHECK((sale_price IS NULL AND for_sale_since IS NULL) OR (sale_price IS NOT NULL AND for_sale_since IS NOT NULL)), /* make sure that sale_price and for_sale_since are in sync */
			FOREIGN KEY(user_id)         REFERENCES users(user_id)                       ON UPDATE CASCADE ON DELETE RESTRICT, /* cannot delete a user with inventory */
			FOREIGN KEY(listing_id)      REFERENCES listings(listing_id)                 ON UPDATE CASCADE ON DELETE RESTRICT, /* cannot delete a listing that someone still has */
			FOREIGN KEY(withdrawal_code) REFERENCES pending_withdrawals(withdrawal_code) ON UPDATE CASCADE ON DELETE SET NULL

		);
		CREATE INDEX IF NOT EXISTS slotownerindex ON slots(user_id, slot_index);
		CREATE INDEX IF NOT EXISTS slotowner      ON slots(user_id);
		CREATE INDEX IF NOT EXISTS slotlisting    ON slots(listing_id);
		CREATE INDEX IF NOT EXISTS slotwithdrawal ON slots(withdrawal_code); /* the table is queried by this column once */`)
		if err != nil {
			log.Println("Unable to create slots table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS listing_buy_orders (

			user_id    INTEGER NOT NULL,                                 /* which user placed this buy order */
			listing_id INTEGER NOT NULL,                                 /* which listing the buy order is in */
			quantity   INTEGER NOT NULL,                                 /* how many they're willing to buy */
			price      INTEGER NOT NULL,                                 /* how much they're willing to pay for each one */
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), /* when this buy order was created */

			UNIQUE(user_id, listing_id, price),  /* can't have two buy orders open for the same item by the same user for the same price */
			CHECK(quantity > 0),
			CHECK(price > 0),
			FOREIGN KEY(user_id)    REFERENCES users(user_id)       ON UPDATE CASCADE ON DELETE CASCADE,
			FOREIGN KEY(listing_id) REFERENCES listings(listing_id) ON UPDATE CASCADE ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS buyowner   ON listing_buy_orders(user_id);
		CREATE INDEX IF NOT EXISTS buylisting ON listing_buy_orders(listing_id);`)
		if err != nil {
			log.Println("Unable to create listing_buy_orders table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS inventory ( /* 100 percent confirmed items that our bots definitely have in echests */

			item_id     INTEGER NOT NULL PRIMARY KEY, /* item name */
			listing_id  INTEGER NOT NULL,             /* which listing this item is. this also determines which server we're talking about since listings are server specific */
			bot_uuid    TEXT    NOT NULL,             /* UUID of the bot that has this item in its ender chest */
			slot_number INTEGER NOT NULL,             /* where in the bot's echest is this, 0 through 26 inclusive */

			/* CANNOT do UNIQUE(bot_uuid,slot_number) because we reuse bots across servers, the same bot in slot 1 of its echest can have a different item on two different servers */
			CHECK(item_id > 0),
			CHECK(LENGTH(bot_uuid) = 36),
			CHECK(slot_number >= 0 AND slot_number < 27),
			FOREIGN KEY(listing_id) REFERENCES listings(listing_id) ON UPDATE CASCADE ON DELETE RESTRICT /* cannot delete a listing that still has inventory */
		);
		CREATE INDEX IF NOT EXISTS inventorylisting ON inventory(listing_id);`)
		if err != nil {
			log.Println("Unable to create inventory table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS pending_deposits (

			deposit_id   INTEGER NOT NULL PRIMARY KEY, /* ID of this deposit, the name of the item will be this in hexadecimal */
			user_id      INTEGER NOT NULL,             /* which user initiated this deposit */
			listing_id   INTEGER NOT NULL,             /* which listing the deposit is into */
			expiry_time  INTEGER NOT NULL,             /* when will this deposit expire if not completed, unix time, seconds since epoch. if a bot picks up the item after this time, it will throw it out */
			picked_up_at INTEGER,                      /* when a bot picked up this deposit, MAYBE. NULL if we've never seen this deposit at all. unix time, seconds since epoch of course. NOT actually confirmed yet! */

			UNIQUE(user_id, expiry_time),   /* a user can't make more than one a second LOL */
			CHECK(expiry_time > 0),
			CHECK(deposit_id > 0),
			CHECK(picked_up_at IS NULL OR picked_up_at > 0),
			FOREIGN KEY(user_id)    REFERENCES users(user_id)       ON UPDATE CASCADE ON DELETE CASCADE,
			FOREIGN KEY(listing_id) REFERENCES listings(listing_id) ON UPDATE CASCADE ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS depositowner ON pending_deposits(user_id);`)
		if err != nil {
			log.Println("Unable to create pending_deposits table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS pending_withdrawals (

			withdrawal_code INTEGER NOT NULL PRIMARY KEY, /* code that causes the bot to drop the item */
			item_id         INTEGER NOT NULL,             /* reference to inventory of which item is being withdrawn from where */
			expiry_time     INTEGER NOT NULL,             /* when will this withdrawal expire if not completex, unix time, seconds since epoch */

			UNIQUE(item_id), /* not possible for two currently pending withdrawals to be on the same specific item */
			CHECK(expiry_time > 0),
			FOREIGN KEY(item_id) REFERENCES inventory(item_id) ON UPDATE CASCADE ON DELETE RESTRICT
		);`)
		if err != nil {
			log.Println("Unable to create pending_withdrawals table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS completed_listing_trades (

			seller_id INTEGER NOT NULL,                                 /* who sold the item */
			buyer_id INTEGER NOT NULL,                                  /* who bought the item */
			listing_id INTEGER NOT NULL,                                /* the item */
			price INTEGER NOT NULL,                                     /* the price */
			timestamp INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), /* when it happened */

			CHECK(price >= 0), /* you can indeed sell something for free */
			CHECK(timestamp > 0),
			CHECK(buyer_id != seller_id),
			FOREIGN KEY(buyer_id)   REFERENCES users(user_id)       ON UPDATE CASCADE ON DELETE RESTRICT, /* TODO figure out what should happen here. */
			FOREIGN KEY(seller_id)  REFERENCES users(user_id)       ON UPDATE CASCADE ON DELETE RESTRICT,
			FOREIGN KEY(listing_id) REFERENCES listings(listing_id) ON UPDATE CASCADE ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS tradebuyer  ON completed_listing_trades(buyer_id);
		CREATE INDEX IF NOT EXISTS tradeseller ON completed_listing_trades(seller_id);
		CREATE INDEX IF NOT EXISTS tradeitem   ON completed_listing_trades(listing_id);`)
		if err != nil {
			log.Println("Unable to create completed_listing_trades table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS currencies (

			currency_id INTEGER NOT NULL PRIMARY KEY,
			currency_name TEXT NOT NULL,

			UNIQUE(currency_name),
			CHECK(LENGTH(currency_name) > 0)
		);
		`)
		if err != nil {
			log.Println("Unable to create currencies table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS balances (

			user_id INTEGER NOT NULL,     /* who has this currency */
			currency_id INTEGER NOT NULL, /* what is the currency */
			balance INTEGER NOT NULL,     /* how much of it do they have */

			CHECK(balance >= 0),
			FOREIGN KEY(user_id)     REFERENCES users(user_id)          ON UPDATE CASCADE ON DELETE RESTRICT,
			FOREIGN KEY(currency_id) REFERENCES currencies(currency_id) ON UPDATE CASCADE ON DELETE RESTRICT
		);
		CREATE INDEX IF NOT EXISTS balancesuser     ON balances(user_id);
		CREATE INDEX IF NOT EXISTS balancescurrency ON balances(currency_id);
		`)
		if err != nil {
			log.Println("Unable to create balances table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS currency_buy_orders (

			user_id     INTEGER NOT NULL,                                 /* which user placed this buy order */
			currency_id INTEGER NOT NULL,                                 /* which currency the buy order is for */
			quantity    INTEGER NOT NULL,                                 /* how many they're willing to buy */
			price       INTEGER NOT NULL,                                 /* how much they're willing to pay for each one */
			created_at  INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), /* when this buy order was created */

			UNIQUE(user_id, currency_id, price),  /* can't have two buy orders open for the same currency by the same user for the same price */
			CHECK(quantity > 0),
			CHECK(price > 0),
			FOREIGN KEY(user_id)     REFERENCES users(user_id)          ON UPDATE CASCADE ON DELETE CASCADE,
			FOREIGN KEY(currency_id) REFERENCES currencies(currency_id) ON UPDATE CASCADE ON DELETE CASCADE
		);`)
		if err != nil {
			log.Println("Unable to create currency_buy_orders table")
			return err
		}
		_, err = sql.Exec(`CREATE TABLE IF NOT EXISTS currency_sell_orders (

			user_id     INTEGER NOT NULL,                                 /* which user placed this sell order */
			currency_id INTEGER NOT NULL,                                 /* which currency the sell order is for */
			quantity    INTEGER NOT NULL,                                 /* how many they're willing to sell (of currency, not of RE) */
			price       INTEGER NOT NULL,                                 /* how much they want to receive for each one */
			created_at  INTEGER NOT NULL DEFAULT (strftime('%s', 'now')), /* when this sell order was created */

			UNIQUE(user_id, currency_id, price),  /* can't have two sell orders open for the same currency by the same user for the same price */
			CHECK(quantity > 0),
			CHECK(price > 0),
			FOREIGN KEY(user_id)     REFERENCES users(user_id)          ON UPDATE CASCADE ON DELETE CASCADE,
			FOREIGN KEY(currency_id) REFERENCES currencies(currency_id) ON UPDATE CASCADE ON DELETE CASCADE
		);`)
		if err != nil {
			log.Println("Unable to create currency_sell_orders table")
			return err
		}
		return nil
	})
	if err != nil {
		panic(err) // immediately quit if we cannot create our tables
	}
	log.Println("Database setup completed")
}
