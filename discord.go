package main

import (
	"errors"
	"log"
	"os"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

const Currency = "R€" // i literally hate pasting the euro symbol everywhere so im making it a constant

var discord *discordgo.Session

func setupDiscordBot() {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		panic("Must set environment variable DISCORD_BOT_TOKEN")
	}
	log.Println("Establishing discord connection")
	var err error
	discord, err = discordgo.New("Bot " + token)
	if err != nil {
		panic(err)
	}
	err = discord.Open()
	if err != nil {
		panic(err)
	}
	log.Println("Connected to discord")
}

func DMuser(user_id int64, message string) error {
	if discord == nil {
		return errors.New("Discord not connected!")
	}
	userIdAsStr := strconv.FormatInt(user_id, 10) // base 10 lol
	log.Println("Attempting to DM the message\"", message, "\" to user "+userIdAsStr)
	ch, err := discord.UserChannelCreate(userIdAsStr) // only creates it if it doesn't already exist
	if err != nil {
		return err
	}
	_, err = discord.ChannelMessageSend(ch.ID, message)
	return err
}

func (user User) DM(message string) error { // just a fancy receiver wrapper
	return DMuser(user.UserID, message)
}
