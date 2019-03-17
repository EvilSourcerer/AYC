package main

import (
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/pat"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	gothDiscord "github.com/markbates/goth/providers/discord"
)

type User struct { // someone's information, as received from discord oauth
	UserID    int64
	Name      string
	AvatarURL string
}

const OurCookieName = "2b2tq"
const HttpOnly = true             // change this to false in production with https support
const ProfileDataSection = "user" // we store our user info in a section of the session called "user"
const CurrentDomainName = "http://localhost:3000"
const CallbackURL = CurrentDomainName + "/auth/discord/callback" // this must equal the callback serve down below, but with discord replaced with {provider}
const HomeURL = "/home"                                          // where a logged in user goes

const discordID2b2tq = "510930252676071439"

var sessionStore sessions.Store

// weird shit dont touch its radioactive
func init() {
	gob.Register(User{}) // this allows it to be saved in the session
}

// weird shit dont touch its radioactive

func setupSession() {
	sessionSecret := os.Getenv("SESSION_SECRET") // note that gothic uses the same session secret, the SESSION_SECRET env variable
	// this is fine since we use a diffreent cookie name, "2b2tq", instead of "_gothic_user"
	if sessionSecret == "" {
		panic("Must set environment variable SESSION_SECRET")
	}
	store := sessions.NewCookieStore([]byte(sessionSecret))
	store.Options.HttpOnly = HttpOnly
	sessionStore = store
}

func getDiscordSecrets() (string, string) {
	key := os.Getenv("DISCORD_KEY")
	secret := os.Getenv("DISCORD_SECRET")
	if key == "" || secret == "" {
		panic("Must set environment variables DISCORD_KEY and DISCORD_SECRET")
	}
	return key, secret
}

func setupDiscord() {
	key, secret := getDiscordSecrets()
	goth.UseProviders(gothDiscord.New(key, secret, CallbackURL, gothDiscord.ScopeIdentify))
}

func setupAuth(p *pat.Router) {
	setupSession()
	setupDiscord()

	p.Get("/logout", func(res http.ResponseWriter, req *http.Request) {
		session, _ := sessionStore.Get(req, OurCookieName)
		session.Options.MaxAge = -1 // apparently this is how you're supposed to delete a session
		session.Save(req, res)
		http.Redirect(res, req, "/", http.StatusFound)
	})

	p.Get("/auth/{provider}/callback", func(res http.ResponseWriter, req *http.Request) {
		gothUser, err := gothic.CompleteUserAuth(res, req)
		if err != nil {
			fmt.Fprintln(res, err)
			return
		}
		// goth library has gotten us their verified discord information
		// now to save it in the session cookie...

		user, err := initUser(gothUser)
		if err != nil {
			// discord gave invalid data...?
			fmt.Fprintln(res, err)
			return
		}

		session, _ := sessionStore.Get(req, OurCookieName)
		session.Values[ProfileDataSection] = *user // save as a User not a *User, we know it's not nil since we checked the error above
		session.Save(req, res)
		http.Redirect(res, req, HomeURL, http.StatusFound)
	})

	p.Get("/auth/{provider}", func(res http.ResponseWriter, req *http.Request) {
		if getUser(req) != nil {
			// already logged in
			http.Redirect(res, req, HomeURL, http.StatusFound)
			return
		}
		gothic.BeginAuthHandler(res, req)
	})
}

func getUser(req *http.Request) *User {
	session, _ := sessionStore.Get(req, OurCookieName)
	// ignore error since .Get is guaranteed to return a session, just an empty one if the cookie is invalid
	user, ok := session.Values[ProfileDataSection].(User)
	if !ok || user.UserID == 0 {
		return nil
	}
	return &user
}

func initUser(gothUser goth.User) (*User, error) { // take the information we got from discord about their user, and convert it into the format we want
	UserIDstr := gothUser.UserID

	UserID, err := strconv.ParseInt(UserIDstr, 10, 64) // base 10, 64 bits
	if err != nil || UserID <= 0 {
		// discord gave us a bad user id?
		return nil, err
	}

	membership, err := discord.GuildMember(discordID2b2tq, UserIDstr)
	if err != nil || membership == nil {
		return nil, errors.New("You must be a member of the 2b2tq discord server")
	}
	log.Println(*membership)

	err = RunSQL(func(sql *sql.Tx) error {
		_, err := sql.Exec("INSERT OR IGNORE INTO users (user_id) VALUES (?)", UserID)
		return err
	})
	if err != nil {
		// there is literally no reason why that could ever fail.
		// but shrug. maybe..... the disk is full and it can't save even just one more row to disk. lol
		return nil, err
	}
	return &User{
		UserID:    UserID,
		Name:      gothUser.Name,
		AvatarURL: gothUser.AvatarURL,
	}, nil // nil error = worked fine, no error
}
