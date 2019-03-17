package main

import (
	"database/sql"
	"strconv"
)

type NavInfo struct {
	ItemInfo Listing
	Bid      string
	Ask      string
}

type Navigation []NavInfo

func generateNavigation() Navigation {
	var result Navigation
	err := RunSQL(func(sql *sql.Tx) error {
		rows, err := sql.Query(`SELECT listings.listing_id, listings.server, listings.item_name, listings.item_photo, COALESCE(MAX(listing_buy_orders.price), -1) AS buy_order_max, COALESCE(MIN(slots.sale_price), -1) AS sell_order_min FROM listings LEFT OUTER JOIN listing_buy_orders ON listings.listing_id = listing_buy_orders.listing_id LEFT OUTER JOIN (SELECT * FROM slots WHERE sale_price IS NOT NULL) slots ON listings.listing_id = slots.listing_id GROUP BY listings.listing_id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var info NavInfo
			var bid int64
			var ask int64
			err = rows.Scan(&info.ItemInfo.ListingID, &info.ItemInfo.Server, &info.ItemInfo.ItemName, &info.ItemInfo.ItemPhoto, &bid, &ask)
			if err != nil {
				return err
			}
			if bid >= 0 { // -1 is "no orders"
				info.Bid = strconv.FormatInt(bid, 10) + Currency
			} else {
				info.Bid = "--"
			}
			if ask >= 0 {
				info.Ask = strconv.FormatInt(ask, 10) + Currency
			} else {
				info.Ask = "--"
			}
			result = append(result, info)
		}
		return rows.Err()
	})
	if err != nil {
		panic(err)
	}
	return result
}
