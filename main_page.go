package main

import (
	"database/sql"
	"net/http"
)

type MainPageTemplate struct { // this struct represents the data that is passed to "template/index.html" to render it
	Navigation  Navigation
	Navbar      string
	Profile     *User
	Balance     int
	Statistics  string
	BotStatuses []BotStatus
}

func handleMainPage(w http.ResponseWriter, r *http.Request) { // handle a request to the main page
	data := &MainPageTemplate{
		Navigation:  generateNavigation(),
		Profile:     getUser(r), // call getUser in serve.go to get user info
		Balance:     0,
		Statistics:  "yep statistics",
		BotStatuses: GetBotStatuses(),
	}
	if data.Profile != nil {

		// we can grab their balance if their profile isn't nil
		// note that their profile is just getUser(request), which returns nil if they are not logged in

		// this function runs a SQL query in a completely guaranteed to be safe manner
		err := RunSQL(func(sql *sql.Tx) error {
			row := sql.QueryRow("SELECT balance FROM users WHERE user_id = ?", data.Profile.UserID)
			err := row.Scan(&data.Balance) // this "scans" one row from the result of that query into a pointer to data.Balance
			// this is how the result of the query "gets outside" this nested function definition
			// from inside here, we can change the value of variables from outside
			// for example, it's valid to do something like "data.Balance = 0" from inside here
			// but any variables we define for the first time, in here, can't be used outside

			return err // Note that if this internal function returns a non-nil error, it rolls back every SQL command that was run in here
			// the magic of sqlite database commits and rollbacks
			// in this case, all we did was a select, so there's nothing to roll back
			// but if it was more complicated, with inserts, if this function returns an error at the end, it undoes all of them and puts everything back how it was
		})

		if err != nil {
			w.Write([]byte("<script>alert('unable to fetch your balance');</script>"))
			err = nil
		}
	}
	err := templates.ExecuteTemplate(w, "index.html", data) // render the index.html template, filling it in with the data
	if err != nil {
		http.Error(w, "Unable to render the main page template. ", http.StatusInternalServerError)
		return
	}
}
