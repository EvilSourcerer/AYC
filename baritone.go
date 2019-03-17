package main

import (
	"log"
	"net"
	"sync"
	"time"
)

var bots []*Bot
var botsLock sync.Mutex

type BotStatus struct {
	timeReceivedUnixNano    int64
	BotUUID                 string
	ServerIP                string
	X                       float64
	Y                       float64
	Z                       float64
	Yaw                     float32
	Pitch                   float32
	OnGround                bool
	Health                  float32
	Saturation              float32
	FoodLevel               int
	Dimension               int
	PathStartX              int
	PathStartY              int
	PathStartZ              int
	HasCurrentSegment       bool
	HasNextSegment          bool
	CalcInProgress          bool
	TicksRemainingInCurrent float64
	CalcFailedLastTick      bool
	SafeToCancel            bool
	CurrentGoal             string
	CurrentProcess          string
	MainInventory           []string
	Armor                   []string
	OffHand                 string
	WindowId                int
	EChestOpenNow           bool
}

type Bot struct {
	latestStatus *BotStatus
	conn         net.Conn
}

func (bs BotStatus) AgeMillis() int64 { // how many milliseconds ago was this bot status received
	return (time.Now().UnixNano() - bs.timeReceivedUnixNano) / 1000000
}

func GetBotStatuses() []BotStatus { // get statuses of all currently active bots
	botsLock.Lock()
	defer botsLock.Unlock()
	statuses := make([]BotStatus, 0)
	for _, bot := range bots {
		if bot.latestStatus != nil {
			statuses = append(statuses, *bot.latestStatus)
		}
	}
	return statuses
}

func baritoneListen() { // listen for connections from baritone bots
	l, err := net.Listen("tcp", ":5021")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	log.Println("Listening for baritowones")
	for {
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		}
		go handleNewBot(conn)
	}
}

func handleNewBot(conn net.Conn) { // handle a new incoming connection that was made by a bot to us
	botsLock.Lock()
	defer botsLock.Unlock()
	bot := &Bot{
		latestStatus: nil,
		conn:         conn,
	}
	bots = append(bots, bot)
	go bot.handleMessages()
}

func (bot *Bot) handleMessages() { // handle incoming messages coming from a given bot
	for {
		msgType := bot.readByte()
		switch msgType {
		case 0:
			log.Println("status packet uwu")
			bot.readStatusPacket()
			log.Println()
		case 4:
			log.Println("echest packet uwu")
			bot.readEchestPacket()
			log.Println()
		default:
			log.Println("oh noes! unknown packet type", msgType, "what do :S")
			bot.conn.Close() // just destroy everything xdx xdd
		}
	}
}

func (bot *Bot) readStatusPacket() { // read a bot's status
	status := &BotStatus{
		timeReceivedUnixNano:    time.Now().UnixNano(),
		BotUUID:                 bot.readUTF(),
		ServerIP:                bot.readUTF(),
		X:                       bot.readDouble(),
		Y:                       bot.readDouble(),
		Z:                       bot.readDouble(),
		Yaw:                     bot.readFloat(),
		Pitch:                   bot.readFloat(),
		OnGround:                bot.readBoolean(),
		Health:                  bot.readFloat(),
		Saturation:              bot.readFloat(),
		FoodLevel:               bot.readInt(),
		Dimension:               bot.readInt(),
		PathStartX:              bot.readInt(),
		PathStartY:              bot.readInt(),
		PathStartZ:              bot.readInt(),
		HasCurrentSegment:       bot.readBoolean(),
		HasNextSegment:          bot.readBoolean(),
		CalcInProgress:          bot.readBoolean(),
		TicksRemainingInCurrent: bot.readDouble(),
		CalcFailedLastTick:      bot.readBoolean(),
		SafeToCancel:            bot.readBoolean(),
		CurrentGoal:             bot.readUTF(),
		CurrentProcess:          bot.readUTF(),
		MainInventory:           bot.readManyStrings(36),
		Armor:                   bot.readManyStrings(4),
		OffHand:                 bot.readUTF(),
		WindowId:                bot.readInt(),
		EChestOpenNow:           bot.readBoolean(),
	}
	bot.latestStatus = status
	bot.onBotInventoryUpdate()
	log.Println("INvy", bot.latestStatus.MainInventory)
}

func (bot *Bot) readEchestPacket() {
	slot := bot.readInt()
	item := bot.readUTF()
	log.Println("They have", item, "in slot", slot)
	shouldDrop := botHasItemInEchest(slot, item, bot.latestStatus.BotUUID, "2b2t.org")
	if !shouldDrop {
		return
	}
	go func() {
		time.Sleep(125 * time.Millisecond)
		log.Println("DrOpPiNg")
		if bot.latestStatus.EChestOpenNow {
			log.Println("okay this dude is open")
			// bot.latestStatus.WindowId is therefore guaranteed to refer to the echest
			bot.sendWindowClick(bot.latestStatus.WindowId, slot, 1, THROW)
		}
	}()
}

func (bot *Bot) hasReceivedStatusUpdateInTheLastFiveSeconds() bool {
	if bot.latestStatus == nil {
		return false
	}
	return time.Now().UnixNano()-bot.latestStatus.timeReceivedUnixNano < int64(5*time.Second)
}

func goToEnderChest() {
	// lol
	botsLock.Lock()
	defer botsLock.Unlock()
	bots[0].sendChatControl("goto ender_chest") // lol
}

func (bot *Bot) onBotInventoryUpdate() {
	log.Println("Reported server ip", bot.latestStatus.ServerIP)
	for i, str := range bot.latestStatus.MainInventory {
		keep := botHasItemInInventory(i, str, bot.latestStatus.BotUUID, "2b2t.org")
		if str != "empty" {
			slot := i
			if bot.latestStatus.EChestOpenNow {
				if slot < 9 {
					slot += 27 // minecraft makes no sense
				}
				slot += 27
			} else {
				if slot < 9 {
					slot += 36 // minecraft makes literally NO sense
				}
			}
			if keep {
				// store in echest, shift click aka QUICK_MOVE it
				if bot.latestStatus.EChestOpenNow { // can only stash in echest if echest is open
					// TODO should we drop valid deposit items that we pick up on the way to an echest, but before arrival
					bot.sendWindowClick(bot.latestStatus.WindowId, slot, 0, QUICK_MOVE)
					break // only do one at a time
				} else {
					goToEnderChest()
				}
			} else {
				bot.sendWindowClick(bot.latestStatus.WindowId, slot, 1, THROW)
				break
			}
		}
	}
}

func getByUUIDAndServer(uuid string, server string) *Bot {
	botsLock.Lock()
	defer botsLock.Unlock()
	for _, bot := range bots {
		if bot.latestStatus == nil {
			continue
		}
		if bot.latestStatus.BotUUID == uuid && bot.latestStatus.ServerIP == server {
			return bot
		}
	}
	return nil
}

func getConnectedToServer(server string) []*Bot {
	botsLock.Lock()
	defer botsLock.Unlock()
	result := make([]*Bot, 0)
	for _, bot := range bots {
		if bot.latestStatus == nil {
			continue
		}
		if bot.latestStatus.ServerIP == server {
			result = append(result, bot)
		}
	}
	return result
}
