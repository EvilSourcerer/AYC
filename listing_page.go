package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
)

type OrderBook struct {
	Count     int
	Bestprice string
}

type MarketStatus struct {
	Buys  OrderBook
	Sells OrderBook
}

type ListingTemplate struct {
	Navigation  Navigation
	Info        MarketStatus
	ItemInfo    Listing
	Profile     *User
	Balance     int
	Statistics  string
	BotStatuses []BotStatus
}

func currentMarketStatus(listing_id int64) (MarketStatus, error) {
	var result MarketStatus
	err := RunSQL(func(sql *sql.Tx) error {
		row := sql.QueryRow("SELECT COUNT(*), COALESCE(MIN(sale_price), 0) FROM slots WHERE listing_id = ? AND sale_price IS NOT NULL", listing_id)
		var sellOrdersCount int
		var minSalePrice int64
		err := row.Scan(&sellOrdersCount, &minSalePrice)
		if err != nil {
			return err
		}
		result.Sells.Count = sellOrdersCount
		result.Sells.Bestprice = strconv.FormatInt(minSalePrice, 10) + Currency

		row = sql.QueryRow("SELECT COALESCE(SUM(quantity), 0), COALESCE(MAX(price), 0) FROM listing_buy_orders WHERE listing_id = ?", listing_id)
		var buyOrdersCount int
		var maxBuyPrice int64
		err = row.Scan(&buyOrdersCount, &maxBuyPrice)
		if err != nil {
			return err
		}
		result.Buys.Count = buyOrdersCount
		result.Buys.Bestprice = strconv.FormatInt(maxBuyPrice, 10) + Currency
		return nil
	})
	return result, err
}

func handleListing(w http.ResponseWriter, r *http.Request) { // handle a request to the main page
	listingIdStr := r.URL.Query().Get(":listing")
	log.Println("Listing id str", listingIdStr)
	listingId, err := strconv.ParseInt(listingIdStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid listing "+err.Error(), http.StatusInternalServerError)
		return
	}
	listing := getListingById(listingId)
	log.Println("Request to listing", listing)
	if listing == nil {
		http.Error(w, "Invalid listing", http.StatusInternalServerError)
		return
	}
	status, err := currentMarketStatus(listingId)
	if err != nil {
		http.Error(w, "Invalid listing "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := &ListingTemplate{
		Navigation:  generateNavigation(),
		Info:        status,
		ItemInfo:    *listing,
		Profile:     getUser(r), // call getUser in serve.go to get user info
		Balance:     0,
		Statistics:  "idk xd",
		BotStatuses: GetBotStatuses(),
	}
	if data.Profile != nil {
		err := RunSQL(func(sql *sql.Tx) error {
			return sql.QueryRow("SELECT balance FROM users WHERE user_id = ?", data.Profile.UserID).Scan(&data.Balance)
		})

		if err != nil {
			http.Error(w, "Unable to fetch your balance. "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	err = templates.ExecuteTemplate(w, "listing.html", data)
	if err != nil {
		http.Error(w, "Unable to render the listing page template. "+err.Error(), http.StatusInternalServerError)
		return
	}
}
