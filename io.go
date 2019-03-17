package main

import (
	"encoding/binary"
)

// aka i'm so used to datainputstream i made it in go

func (bot *Bot) readUTF() string {
	l := bot.readShort()
	data := make([]byte, l)
	err := binary.Read(bot.conn, binary.BigEndian, &data)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func (bot *Bot) readDouble() float64 {
	var data float64
	err := binary.Read(bot.conn, binary.BigEndian, &data)
	if err != nil {
		panic(err)
	}
	return data
}

func (bot *Bot) readFloat() float32 {
	var data float32
	err := binary.Read(bot.conn, binary.BigEndian, &data)
	if err != nil {
		panic(err)
	}
	return data
}

func (bot *Bot) readInt() int {
	var data int32
	err := binary.Read(bot.conn, binary.BigEndian, &data)
	if err != nil {
		panic(err)
	}
	return int(data)
}

func (bot *Bot) readShort() uint16 {
	var data uint16
	err := binary.Read(bot.conn, binary.BigEndian, &data)
	if err != nil {
		panic(err)
	}
	return data
}

func (bot *Bot) readByte() uint8 {
	var data uint8
	err := binary.Read(bot.conn, binary.BigEndian, &data)
	if err != nil {
		panic(err)
	}
	return data
}

func (bot *Bot) readBoolean() bool {
	return bot.readByte() != 0
}

func (bot *Bot) readManyStrings(num int) []string {
	data := make([]string, num)
	for i := 0; i < num; i++ {
		data[i] = bot.readUTF()
	}
	return data
}

func (bot *Bot) sendChatControl(str string) { // send chat control to a bot
	err := binary.Write(bot.conn, binary.BigEndian, uint8(1))
	if err != nil {
		panic(err)
	}
	err = binary.Write(bot.conn, binary.BigEndian, uint16(len(str)))
	if err != nil {
		panic(err)
	}
	err = binary.Write(bot.conn, binary.BigEndian, []byte(str))
	if err != nil {
		panic(err)
	}
}

func (bot *Bot) writeByte(byte uint8) {
	err := binary.Write(bot.conn, binary.BigEndian, byte) // remember, on a 64-bit system int means int64 so gotta make sure to only send a 4-byte int because that's what java expects here
	if err != nil {
		panic(err)
	}
}

func (bot *Bot) writeInt(i int) {
	err := binary.Write(bot.conn, binary.BigEndian, int32(i)) // remember, on a 64-bit system int means int64 so gotta make sure to only send a 4-byte int because that's what java expects here
	if err != nil {
		panic(err)
	}
}

type ClickType int

const (
	PICKUP ClickType = iota
	QUICK_MOVE
	SWAP
	CLONE
	THROW
	QUICK_CRAFT
	PICKUP_ALL
)

// really cool fancy schmancy go feature
// for example, PICKUP is 0 and THROW is 4

func (bot *Bot) sendWindowClick(windowId int, slotId int, mouseButton int, clickType ClickType) {
	bot.writeByte(uint8(5))
	bot.writeInt(windowId)
	bot.writeInt(slotId)
	bot.writeInt(mouseButton)
	bot.writeInt(int(clickType)) // clickType is an "int" but go won't convert automatically for safety
}
