package main // why aren't you compiling

import (
	"context"
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/pat"
)

var server *http.Server
var templates = parseTemplates() // automatically loads templates! HAX!
func parseTemplates() *template.Template {
	templ := template.New("")
	err := filepath.Walk("template/", func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, ".html") {
			_, err = templ.ParseFiles(path)
			if err != nil {
				return err
			}
		}

		return err
	})

	if err != nil {
		panic(err)
	}

	return templ
}

func handleEnderChest(w http.ResponseWriter, r *http.Request) { // handle a request to ender_chest
	log.Println("Going to an ENDER CHEST OWO")
	goToEnderChest()                           // lol
	http.Redirect(w, r, "/", http.StatusFound) // redirect back to main page, 302 temporary redirect
}

func handleFreeRE(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		http.Error(w, "not logged in LOL", http.StatusInternalServerError)
		return
	}
	err := RunSQL(func(sql *sql.Tx) error {
		_, err := sql.Exec("UPDATE users SET balance = balance + 1 WHERE user_id = ?", user.UserID)
		// this query returns a result, which I think is their new balance, but we ignore it, we just care about if there was an error
		return err
	})
	if err != nil {
		http.Error(w, "Unable to execute SQL to increment your balance. "+err.Error(), http.StatusInternalServerError)
		return
	}
	err = user.DM("You just claimed 1 free " + Currency + "!")
	if err != nil {
		log.Println(err)
	}
	http.Redirect(w, r, "/", http.StatusFound) // redirect back to main page, 302 temporary redirect
}

func serve() {
	p := pat.New() // this is a simple library (imported above), pat is short for pattern, and it makes it easier to do routes (get / post / etc)

	setupAuth(p) // setup login with discord and logout, calls auth.go

	// this is where all the webserver files are located!

	p.Get("/ender_chest", handleEnderChest) // going to /ender_chest should do the ender chest thing
	p.Get("/freere", handleFreeRE)          // just for testing
	p.Get("/trade/{listing}", handleListing)
	p.Get("/dashboard", handleDashboardPage)
	p.Get("/categories", handleCategories)
	p.Get("/neworders", handleorders)
	p.Get("/market", handlemarket)
	p.Get("/", handleMainPage) // going to / should render the main page

	// this is where all the unchanging things are!

	http.Handle("/assets/", http.FileServer(http.Dir("."))) // any request that begins with "/assets/" will be a file server, this is for css and js and image files in static/

	http.Handle("/", p)

	server = &http.Server{
		Addr: ":3000",
	}
	log.Println("Listening for HTTP...")
	log.Fatal(server.ListenAndServe()) // if serve() returns an error, this prints it out
}

func ShutdownHTTP() {
	// this is just something I pasted from stackoverflow to shut down a HTTP server cleanly

	// the reason why we do this is so that requests don't get corrupted
	// i.e. you make a request to put in a trade, it goes through the database properly
	// but then at that moment the database and server gets shut down
	// and it gives you an error
	// this shuts it down cleanly

	// specifically, this makes it stop accepting new incoming connections
	// finish responding to current connections
	// then shut down entirely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Println("Wowzers I cannot shut down this http server =(")
		log.Println(err)
	}
}
