package main

import (
	"database/sql"
	"testing"
)

func createSomeExampleUsers(t *testing.T) {
	createInitialListings()
	err := RunSQL(func(sql *sql.Tx) error {
		_, err := sql.Exec("INSERT INTO users (user_id, balance) VALUES (1, 100); INSERT INTO users (user_id, balance) VALUES (2, 100);")
		if err != nil {
			return err
		}
		_, err = sql.Exec("INSERT INTO slots (user_id, slot_index, listing_id, sale_price, for_sale_since) VALUES (?, ?, ?, ?, ?)", 1, 2, 3, 4, 5)
		if err != nil {
			return err
		}
		_, err = sql.Exec("INSERT INTO slots (user_id, slot_index, listing_id) VALUES (?, ?, ?)", 2, 3, 2)
		if err != nil {
			return err
		}
		_, err = sql.Exec("INSERT INTO inventory (item_id, listing_id, bot_uuid, slot_number) VALUES (?, ?, ?, ?)", 6, 3, "51dcd870-d33b-40e9-9fc1-aecdcff96081", 5)
		if err != nil {
			return err
		}
		_, err = sql.Exec("INSERT INTO inventory (item_id, listing_id, bot_uuid, slot_number) VALUES (?, ?, ?, ?)", 7, 2, "51dcd870-d33b-40e9-9fc1-aecdcff96081", 5)
		if err != nil {
			return err
		}

		err = verifyStorage(sql)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}

func TestExampleUsers(t *testing.T) {
	WithTestingDatabase(func() {
		createSomeExampleUsers(t)
	})
}

func TestCantBuyAgainstSelf(t *testing.T) {
	WithTestingDatabase(func() {
		createSomeExampleUsers(t)
		err := RunSQL(func(sql *sql.Tx) error {
			return createBuyOrder(sql, 1, 3, 69, 1)
		})
		if err.Error() != "Cannot place a buy order that would match against one of your own sell orders" {
			t.Error(err)
		}
	})
}

func TestCanBuyFromAnother(t *testing.T) {
	WithTestingDatabase(func() {
		createSomeExampleUsers(t)
		err := RunSQL(func(sql *sql.Tx) error {
			return createBuyOrder(sql, 2, 3, 69, 1)
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var i int64
			err := sql.QueryRow("SELECT COUNT(*) FROM completed_listing_trades").Scan(&i)
			if err != nil {
				return err
			}
			if i != 1 {
				t.Errorf("Did not complete transaction")
			}

			err = sql.QueryRow("SELECT COUNT(*) FROM slots WHERE user_id = 1").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("User 1 somehow still has the sold item")
			}

			err = sql.QueryRow("SELECT COUNT(*) FROM slots WHERE user_id = 2").Scan(&i)
			if err != nil {
				return err
			}
			if i != 2 {
				t.Errorf("User 2 did not receive the item")
			}

			err = sql.QueryRow("SELECT COUNT(*) FROM listing_buy_orders").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("Somehow a unfulfilled buy order was created")
			}

			// even though the buy was for 69 RE each, the sell was for 4 each, so that's the price the trade should have gone through at
			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 1").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100+4 {
				t.Errorf("User 1 did not receive the RE")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 2").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100-4 {
				t.Errorf("User 2 did not lose the RE")
			}

			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})
}

func TestSellAgainstSingleBuy(t *testing.T) {
	WithTestingDatabase(func() {
		createSomeExampleUsers(t)
		err := RunSQL(func(sql *sql.Tx) error {
			return createBuyOrder(sql, 2, 3, 2, 1)
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var i int64
			err := sql.QueryRow("SELECT COUNT(*) FROM completed_listing_trades").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("That should not have bought")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 1").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100 {
				t.Errorf("User 1 should not have had a balance change even though they are selling this listing")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 2").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100-2 {
				t.Errorf("User 2 didn't have the balance taken away wtf")
			}

			return nil
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			return createSellOrder(sql, 1, 2, 3) // shouldn't execute
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var i int64
			err := sql.QueryRow("SELECT COUNT(*) FROM completed_listing_trades").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("That should not have sold")
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			return createSellOrder(sql, 1, 2, 1) // should execute
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var i int64
			err := sql.QueryRow("SELECT COUNT(*) FROM completed_listing_trades").Scan(&i)
			if err != nil {
				return err
			}
			if i != 1 {
				t.Errorf("Didn't complete?")
			}

			err = sql.QueryRow("SELECT COUNT(*) FROM listing_buy_orders").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("Buy order wasn't deleted")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 1").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100+2 {
				t.Errorf("User 1 should have gotten the money")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 2").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100-2 {
				t.Errorf("User 2 didn't have the balance taken away wtf")
			}

			err = sql.QueryRow("SELECT COUNT(*) FROM slots WHERE user_id = 1").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("User 1 somehow still has the sold item")
			}

			err = sql.QueryRow("SELECT COUNT(*) FROM slots WHERE user_id = 2").Scan(&i)
			if err != nil {
				return err
			}
			if i != 2 {
				t.Errorf("User 2 did not receive the item")
			}

			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})
}

func TestSellAgainstMultiBuy(t *testing.T) {
	WithTestingDatabase(func() {
		createSomeExampleUsers(t)
		err := RunSQL(func(sql *sql.Tx) error {
			err := createBuyOrder(sql, 2, 3, 2, 5)
			return err
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var i int64
			err := sql.QueryRow("SELECT COUNT(*) FROM completed_listing_trades").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("That should not have bought")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 1").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100 {
				t.Errorf("User 1 should not have had a balance change even though they are selling this listing")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 2").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100-2*5 {
				t.Errorf("User 2 didn't have the balance taken away wtf")
			}

			return nil
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			return createSellOrder(sql, 1, 2, 3) // shouldn't execute
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var i int64
			err := sql.QueryRow("SELECT COUNT(*) FROM completed_listing_trades").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("That should not have sold")
			}
			return nil
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			return createSellOrder(sql, 1, 2, 1) // should execute
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var i int64
			err := sql.QueryRow("SELECT COUNT(*) FROM completed_listing_trades").Scan(&i)
			if err != nil {
				return err
			}
			if i != 1 {
				t.Errorf("Didn't complete?")
			}

			err = sql.QueryRow("SELECT COUNT(*) FROM listing_buy_orders").Scan(&i)
			if err != nil {
				return err
			}
			if i != 1 {
				t.Errorf("Buy order was deleted")
			}

			err = sql.QueryRow("SELECT quantity FROM listing_buy_orders").Scan(&i)
			if err != nil {
				return err
			}
			if i != 4 {
				t.Errorf("Buy order quantity wasn't decremented")
			}

			err = sql.QueryRow("SELECT price FROM listing_buy_orders").Scan(&i)
			if err != nil {
				return err
			}
			if i != 2 {
				t.Errorf("Buy order price was changed")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 1").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100+2 {
				t.Errorf("User 1 should have gotten the money")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 2").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100-2*5 {
				t.Errorf("User 2 didn't have the balance taken away wtf")
			}

			err = sql.QueryRow("SELECT COUNT(*) FROM slots WHERE user_id = 1").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("User 1 somehow still has the sold item")
			}

			err = sql.QueryRow("SELECT COUNT(*) FROM slots WHERE user_id = 2").Scan(&i)
			if err != nil {
				return err
			}
			if i != 2 {
				t.Errorf("User 2 did not receive the item")
			}

			return nil
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			return cancelAllBuys(sql, 2)
		})
		if err != nil {
			t.Error(err)
		}
		err = RunSQL(func(sql *sql.Tx) error {
			var i int64
			err := sql.QueryRow("SELECT COUNT(*) FROM listing_buy_orders").Scan(&i)
			if err != nil {
				return err
			}
			if i != 0 {
				t.Errorf("Buy order wasn't deleted")
			}

			err = sql.QueryRow("SELECT balance FROM users WHERE user_id = 2").Scan(&i)
			if err != nil {
				return err
			}
			if i != 100-2 {
				t.Errorf("User 2 didn't get a proper refund for canceling the buy order")
			}

			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})
}
