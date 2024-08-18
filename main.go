package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Greeting represents a simple structure for a greeting message
type Greeting struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// Store greetings in a global slice
var greetings []Greeting
var db *sql.DB
var mu sync.Mutex // to protect concurrent access to the greetings slice

func initDatabase() {
	var err error
	db, err = sql.Open("sqlite3", "./greetings.db")
	if err != nil {
		log.Fatalf("error could not open database: %v", err)
	}

	query := `
CREATE TABLE IF NOT EXISTS greetings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name TEXT,
    last_name TEXT,
    message TEXT,
    timestamp TEXT
)`

	_, err = db.Exec(query)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

// holds data that will be passed to HTML
type PageData struct {
	Title      string
	JavaScript template.JS
	Message    string
}

func main() {

	initDatabase()
	defer db.Close()

	// Serve static files from the "static" directory
	//http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Read the compiled JavaScript code
	jsData, err := os.ReadFile("main.js")
	if err != nil {
		log.Printf("Failed to read javascript file")
		log.Fatalf("Failed to read JavaScript file: %v", err)

	}

	// Load the HTML template from an external file
	tmpl, err := template.ParseFiles("template.html")
	if err != nil {
		log.Fatalf("Failed to parse template file: %v", err)
	}

	// Handle the root path and render the template
	// "/" finds the root of the web server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pageData := PageData{
			Title:      "Go Generated Page",
			JavaScript: template.JS(jsData), // javascript code that is inluded
		}

		// tmpl.Execute(w, pageData) renders HTML page from the info stored in pageData
		if err := tmpl.Execute(w, pageData); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Handle the /greet route
	http.HandleFunc("/greet", func(w http.ResponseWriter, r *http.Request) {
		// checks if request method is POST
		if r.Method == http.MethodPost {

			//
			firstName := r.FormValue("first_name")
			lastName := r.FormValue("last_name")

			if firstName == "" || lastName == "" {
				log.Printf("Validation error missing first and lastnames")
				http.Error(w, "First and last names are required", http.StatusBadRequest)
				return
			}

			message := fmt.Sprintf("Thank you, %s %s! Your greeting has been recorded.", firstName, lastName)

			timestamp := time.Now().Format("2006-01-02 15:04:05")
			// Add the new greeting to the global greetings slice
			mu.Lock()

			_, err := db.Exec("INSERT INTO greetings (first_name, last_name, message, timestamp) VALUES (?, ?, ?, ?)", firstName, lastName, message, timestamp)

			// appends new elemt to greetings slice
			// creates greeting struct
			/* 	greetings = append(greetings, Greeting{
				FirstName: firstName, // assign vale firstName to value FirstName
				LastName:  lastName,
				Message:   fmt.Sprintf("Hello, %s %s!", firstName, lastName),
				Timestamp: time.Now().Format("2006-01-02 15:04:05"),
			}) */

			mu.Unlock()

			if err != nil {
				http.Error(w, "Error could not save greeting", http.StatusInternalServerError)
				return
			}

			pageData := PageData{
				Title:      "Go Generated Page",
				JavaScript: template.JS(jsData),
				Message:    message,
			}

			if err := tmpl.Execute(w, pageData); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	// Handle the /greetings route to return a list of greetings as JSON
	http.HandleFunc("/greetings", func(w http.ResponseWriter, r *http.Request) {
		/* 		if r.Method == http.MethodGet {
		   			// Set the Content-Type header to application/json
		   			w.Header().Set("Content-Type", "application/json")

		   			// Encode the greetings data as JSON and send it as the response
		   			mu.Lock()
		   			if err := json.NewEncoder(w).Encode(greetings); err != nil {
		   				http.Error(w, err.Error(), http.StatusInternalServerError)
		   			}
		   			mu.Unlock() // prevents goroutines from modifying greetings
		   		} else {
		   			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		   		} */

		if r.Method == http.MethodGet {
			rows, err := db.Query("SELECT first_name, last_name, message, timestamp FROM greetings")
			if err != nil {
				http.Error(w, "Failed to fetch greetings", http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			var greetings []Greeting
			for rows.Next() {
				var greeting Greeting
				if err := rows.Scan(&greeting.FirstName, &greeting.LastName, &greeting.Message, &greeting.Timestamp); err != nil {
					http.Error(w, "Failed to scan row", http.StatusInternalServerError)
					return
				}
				greetings = append(greetings, greeting)
			}

			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(greetings); err != nil {
				http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			}
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}

	})

	// Handle the /clear route to clear the greetings log
	http.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		/* 	if r.Method == http.MethodPost {
			mu.Lock()
			greetings = []Greeting{} // Clear the log
			// greetings is a slice of Greetings
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		} */
		if r.Method == http.MethodPost {
			mu.Lock()
			_, err := db.Exec("DELETE FROM greetings")
			mu.Unlock()
			if err != nil {
				http.Error(w, "Failed to clear log", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	// Start the HTTP server
	log.Println("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
