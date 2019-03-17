package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"github.com/bdwilliams/go-jsonify/jsonify"
)

// categories for the exchange, e.g. totems, gapples, etc
func handleCategories(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	} else {
		outJSON, err := getJSON("SELECT * FROM listings")
		if err != nil {
			log.Println("wtf lol i couldn't get categories")
		}
		w.Write([]byte(outJSON))
	}
}

func handlemarket(w http.ResponseWriter, r *http.Request) {
	targetlistings := r.URL.Query().Get("category")
	err := RunSQL(func(sql *sql.Tx) error {
		rows, err := sql.Query("SELECT listings.listing_id, listings.server, listings.item_name, listings.item_photo, listings.item_key, COALESCE(MAX(listing_buy_orders.price), -1) AS buy_order_max, COALESCE(MIN(slots.sale_price), -1) AS sell_order_min FROM listings LEFT OUTER JOIN listing_buy_orders ON listings.listing_id = listing_buy_orders.listing_id LEFT OUTER JOIN (SELECT * FROM slots WHERE sale_price IS NOT NULL) slots ON listings.listing_id = slots.listing_id GROUP BY listings.listing_id HAVING listings.item_key = ? AND sell_order_min != -1;", targetlistings)
		if err != nil {
			log.Println(err)
			return err
		}
		defer rows.Close()
		marshalled, err := json.Marshal(jsonify.Jsonify(rows))
		if err != nil {
			log.Println(err)
			return err
		}
		w.Write([]byte(marshalled))
		return err
	})
	if err != nil {
		log.Println(err)
		return
	}
	return
}

func handleorders(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	} else {
		w.Header().Set("Content-Type", "application/json")
		outjson, err := getJSON("SELECT listings.listing_id, listings.server, listings.item_name, listings.item_photo, completed_listing_trades.price, completed_listing_trades.timestamp FROM completed_listing_trades INNER JOIN listings ON completed_listing_trades.listing_id=listings.listing_id ORDER BY completed_listing_trades.timestamp DESC LIMIT 5")
		w.Write([]byte(outjson))
		if err != nil {
			return
		}
	}
}

// WARNING BIG BRAINS CODE FROM STACKOVERFLOW BELOW!!! DO NOT TOUCH!

func getJSON(sqlString string) (string, error) {
	tableData := make([]map[string]interface{}, 0)
	err := RunSQL(func(sql *sql.Tx) error {
		rows, err := sql.Query(sqlString)
		if err != nil {
			return err
		}
		defer rows.Close()
		columns, err := rows.Columns()
		if err != nil {
			return err
		}
		count := len(columns)
		values := make([]interface{}, count)
		valuePtrs := make([]interface{}, count)
		for rows.Next() {
			for i := 0; i < count; i++ {
				valuePtrs[i] = &values[i]
			}
			rows.Scan(valuePtrs...)
			entry := make(map[string]interface{})
			for i, col := range columns {
				var v interface{}
				val := values[i]
				b, ok := val.([]byte)
				if ok {
					v = string(b)
				} else {
					v = val
				}
				entry[col] = v
			}
			tableData = append(tableData, entry)
		}
		return rows.Err()
	})
	if err != nil {
		return "", err
	}
	jsonData, err := json.Marshal(tableData)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}
