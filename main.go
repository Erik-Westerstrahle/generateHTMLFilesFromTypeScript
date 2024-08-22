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
var dataBase *sql.DB
var mu sync.Mutex // to protect concurrent access to the greetings slice

func initDatabase() {
	log.Println("initializing database ")
	var err error
	dataBase, err = sql.Open("sqlite3", "./greetings.dataBase")
	if err != nil {
		log.Fatalf("error could not open database: %v", err)
	}

	// create table greetings if it does not exist
	query := `
CREATE TABLE IF NOT EXISTS greetings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    first_name TEXT,
    last_name TEXT,
    message TEXT,
    timestamp TEXT
)`
	log.Println("table does not exist. Creating table")

	_, err = dataBase.Exec(query)
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
	defer dataBase.Close() // ensures data base is closed when main is running

	// Serve static files from the "static" directory
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Read the compiled JavaScript code
	jsData, err := os.ReadFile("main.js")
	if err != nil {
		log.Printf("Failed to read javascript file")
		log.Fatalf("Failed to read JavaScript file: %v", err)

	}
	log.Println(" loaded Javascript file")

	// Load the HTML template from an external file
	tmpl, err := template.ParseFiles("template.html")
	if err != nil {
		log.Fatalf("Failed to parse template file: %v", err)
	}
	log.Println("Loaded HTML template")

	// Handle the root path and render the template
	// "/" finds the root of the web server
	// w http.ResponseWriter writes to the server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Serving root path request...")
		pageData := PageData{
			Title:      "Go Generated Page",
			JavaScript: template.JS(jsData), // javascript code that is inluded
		}

		// tmpl.Execute(w, pageData) renders HTML page from the info stored in pageData
		if err := tmpl.Execute(w, pageData); err != nil {
			log.Printf("Failed to execute template: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Handle the /greet route
	http.HandleFunc("/greet", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Handling /greet request...")
		// checks if request method is POST
		if r.Method == http.MethodPost {

			// writes the first and lastname to the database file
			firstName := r.FormValue("first_name")
			lastName := r.FormValue("last_name")

			log.Printf("Received firstName: %s, lastName: %s", firstName, lastName) // Debug message

			if firstName == "" || lastName == "" {
				log.Printf("Validation error missing first and lastnames")
				http.Error(w, "First and last names are required", http.StatusBadRequest)
				return
			}

			message := fmt.Sprintf("Thank you, %s %s! Your greeting has been recorded.", firstName, lastName)

			timestamp := time.Now().Format("2006-01-02 15:04:05")
			// Add the new greeting to the global greetings slice

			// mu.Lock() is used to ensure that only go routine can access the database at once
			mu.Lock()
			log.Println("Inserting greeting into database...")
			_, err := dataBase.Exec("INSERT INTO greetings (first_name, last_name, message, timestamp) VALUES (?, ?, ?, ?)", firstName, lastName, message, timestamp)

			mu.Unlock() // unlocks the database

			if err != nil {
				log.Printf("Failed to insert greeting: %v", err)
				http.Error(w, "Error could not save greeting", http.StatusInternalServerError)
				return
			}

			// creates a nw instance of pagedata struct
			pageData := PageData{
				Title:      "Go Generated Page",
				JavaScript: template.JS(jsData),
				Message:    message,
			}

			if err := tmpl.Execute(w, pageData); err != nil {
				log.Printf("Failed to execute template: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				log.Println("Greeting processed and response sent successfully.") // Debug message
			}
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	// Handle the /greetings route to return a list of greetings as JSON
	http.HandleFunc("/greetings", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Handling /greetings request...")

		if r.Method == http.MethodGet {
			rows, err := dataBase.Query("SELECT first_name, last_name, message, timestamp FROM greetings")
			if err != nil {
				log.Printf("Failed to fetch greetings: %v", err)
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
			log.Println("Invalid request method for /greetings")
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}

	})

	// Handle the /clear route to clear the greetings log
	http.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Handling /clear request...")
		// checks if the HTTP method is a post
		if r.Method == http.MethodPost {
			mu.Lock()
			_, err := dataBase.Exec("DELETE FROM greetings")
			mu.Unlock()
			if err != nil {
				log.Printf("Failed to clear log: %v", err)
				http.Error(w, "Failed to clear log", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			log.Println("Greetings log cleared successfully.")
		} else {
			log.Println("Invalid request method for /clear")
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	// Start the HTTP server
	log.Println("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
