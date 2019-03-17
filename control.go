package main

import (
	"log"
	"strconv"
	"strings"
)

const ItemPrefix = "2b2tq.org#"
const ItemNameLength = len(ItemPrefix) + 8 // 8 hex digits == uint32 in hexadecimal

func botHasItemInInventory(pos int, item string, botUUID string, server string) bool {
	listing, id := parseItem(item, server)
	if listing == nil {
		return false
	}
	return botPicksUpItem(listing.ListingID, id)
}

// returns true if bot should drop, false otherwise
func botHasItemInEchest(pos int, item string, botUUID string, server string) bool {
	listing, id := parseItem(item, server)
	log.Println("Got listing and id", listing, "wew", id)
	if listing == nil {
		return false // FOR NOW, do not drop items that don't match that we already have in echest
	}
	log.Println("IN ENDER CHEST", listing, id)
	res := confirmedSavedInEchest(botUUID, pos, listing.ListingID, id)
	log.Println("Result:", res)
	return !res
}

func parseItem(item string, server string) (*Listing, uint32) {
	if item == "empty" {
		return nil, 0
	}
	name, data := extractName(item)
	id := nameToDepositID(name)
	if id == 0 {
		return nil, 0
	}
	if depositIDToName(id) != name {
		// sanity check
		log.Println(id, name)
		return nil, 0
	}
	if len(data) < 40 {
		return nil, 0
	}
	if data[:len("tile.shulkerBox")] != "tile.shulkerBox" {
		return nil, 0
	}
	data = data[strings.Index(data, ";")+6:]
	log.Println("Depsoit ID", id)
	return getListingForContentsOnServer(data, server), id
}

func nameToDepositID(name string) uint32 {
	if len(name) != ItemNameLength {
		return 0
	}
	if name[:len(ItemPrefix)] != ItemPrefix {
		return 0
	}
	d, err := strconv.ParseUint(name[len(ItemPrefix):], 16, 32) // base 16, 32 bit
	if err != nil {
		return 0
	}
	return uint32(d)
}

func depositIDToName(id uint32) string {
	str := strconv.FormatUint(uint64(id), 16)
	for len(str) < 8 {
		str = "0" + str
	}
	str = ItemPrefix + str
	if len(str) != ItemNameLength {
		panic(id)
	}
	return str
}

func extractName(item string) (string, string) {
	ind := strings.Index(item, "$")
	if ind < 0 {
		return "", item
	}
	return item[:ind], item[ind+1:]
}
