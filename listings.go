package main

import (
	"database/sql"
	"log"
)

type Listing struct {
	ListingID int64
	Server    string
	ItemKey   string
	ItemName  string
	ItemPhoto string
}

func getListingById(listing_id int64) *Listing {
	var result Listing
	err := RunSQL(func(sql *sql.Tx) error {
		row := sql.QueryRow("SELECT listing_id, server, item_key, item_name, item_photo FROM listings WHERE listing_id = ?", listing_id)
		return row.Scan(&result.ListingID, &result.Server, &result.ItemKey, &result.ItemName, &result.ItemPhoto)
	})
	if err != nil {
		return nil
	}
	return &result
}

func createInitialListings() {
	// TODO automatically create listings for all item keys on all servers
	// TODO add some more listings, like
	// cooked fish
	// cooked salmon
	// ender chests
	// shulker shells
	// obsidian
	// chorus fruit
	// stacked totems?
	// tnt
	// diamond blocks
	// iron blocks
	// gold blocks
	// emerald blocks
	// coal blocks
	// redstone blocks
	// lapis lazuli blocks

	toCreate := []Listing{
		{0, "2b2t.org", createShulkerFull("item.appleGold;1"), "gapples", "/static/gapple.png"},
		{0, "2b2t.org", createShulkerFull("item.totem;0"), "totems", "/static/totem.png"},
		{0, "2b2t.org", createShulkerFull("item.end_crystal;0"), "end crystals", "/static/endcrystal.png"},
	}
	err := RunSQL(func(sql *sql.Tx) error {
		for _, l := range toCreate {
			_, err := sql.Exec("INSERT OR IGNORE INTO listings (server, item_key, item_name, item_photo) VALUES (?, ?, ?, ?)", l.Server, l.ItemKey, l.ItemName, l.ItemPhoto)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
}

func createShulkerFull(itemWithDamage string) string {
	result := ""
	for i := 0; i < 27; i++ {
		result += itemWithDamage + ";64;,"
	}
	result = result[:len(result)-1] // last comma should be removed
	return result
}

func getListingForContentsOnServer(contents string, server string) *Listing {
	log.Println("CONTENTS", contents)
	log.Println("SERVER", server)
	var result Listing
	err := RunSQL(func(sql *sql.Tx) error {
		row := sql.QueryRow("SELECT listing_id, server, item_key, item_name, item_photo FROM listings WHERE item_key = ? AND server = ?", contents, server)
		return row.Scan(&result.ListingID, &result.Server, &result.ItemKey, &result.ItemName, &result.ItemPhoto)
	})
	if err != nil {
		log.Println("RETURNING ERROR")
		log.Println(err)
		return nil
	}
	log.Println("RETURNING LISTING", result)
	return &result
}
