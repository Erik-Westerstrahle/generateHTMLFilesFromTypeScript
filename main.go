package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
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

// global variables
var greetings []Greeting // this slice store greetings messages
var dataBase *sql.DB     // this connects to database
var mu sync.Mutex        // to protect concurrent access to the greetings slice

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

	// Read the compiled JavaScript code from a file
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

			// error if names are null
			if firstName == "" || lastName == "" {
				log.Printf("Validation error missing first and lastnames")
				http.Error(w, "First and last names are required", http.StatusBadRequest)
				return
			}

			// message to comfirm that the grreting was recorded
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

			// creates a new instance of pagedata struct to pass the template to
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
			defer rows.Close() // closes rows after it has been read

			var greetings []Greeting //This is a slice
			for rows.Next() {
				var greeting Greeting
				if err := rows.Scan(&greeting.FirstName, &greeting.LastName, &greeting.Message, &greeting.Timestamp); err != nil {
					http.Error(w, "Failed to scan row", http.StatusInternalServerError)
					return
				}
				greetings = append(greetings, greeting)
			}

			w.Header().Set("Content-Type", "application/json") // sets response content to JSON
			if err := json.NewEncoder(w).Encode(greetings); err != nil {
				http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			}
		} else {

		}

	})

	// Handle the /clear route to clear the greetings log
	http.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Handling /clear request...")
		// checks if the HTTP method is a post
		if r.Method == http.MethodPost {
			mu.Lock()
			_, err := dataBase.Exec("DELETE FROM greetings") // this executes an SQL delete
			mu.Unlock()
			if err != nil {
				log.Printf("Failed  clear log: %v", err)
				http.Error(w, "Failed  clear log", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			log.Println("Greetings log cleared successfully.")
		} else {

		}
	})

	// Handle the /search route to search for a specific greeting by first and last name
	// "/search" will cause errors
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Handling /search request...")

		if r.Method != http.MethodGet {
			log.Println("Invalid request method for /search")
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		// gets the parameters from URL query string
		firstName := r.URL.Query().Get("first_name")
		lastName := r.URL.Query().Get("last_name")
		startDateStr := r.URL.Query().Get("start_date")
		endDateStr := r.URL.Query().Get("end_date")
		log.Printf("Searching for firstName and %s, lastName: %s, StartDate: %s, EndDate: %s", firstName, lastName, startDateStr, endDateStr) // Debug message

		// initialize start and end dates variables
		var startDate, endDate time.Time
		var err error

		// parse date
		// Parse start_date if provided
		if startDateStr != "" {
			startDate, err = time.Parse("2006-01-02", startDateStr)
			if err != nil {
				log.Printf("Invalid start_date format: %v", err)
				http.Error(w, "Invalid start_date format. Use YYYY-MM-DD.", http.StatusBadRequest)
				return
			}
		}

		// Parse end_date if provided
		if endDateStr != "" {
			endDate, err = time.Parse("2006-01-02", endDateStr)
			if err != nil {
				log.Printf("Invalid end_date format: %v", err)
				http.Error(w, "Invalid end_date format. Use YYYY-MM-DD.", http.StatusBadRequest)
				return
			}
		}

		// SQL query built here
		query := "SELECT first_name, last_name, message, timestamp FROM greetings"
		var queryParams []interface{}
		var conditions []string

		// conditions based on parameters
		if firstName != "" {
			conditions = append(conditions, "first_name = ?") // querys SQL and appends first_name to conditions slice
			queryParams = append(queryParams, firstName)
		}

		if lastName != "" {
			conditions = append(conditions, "last_name = ?")
			queryParams = append(queryParams, lastName)
		}

		if !startDate.IsZero() {
			conditions = append(conditions, "DATE(timestamp) >= ?")
			queryParams = append(queryParams, startDate.Format("2006-01-02"))
		}

		if !endDate.IsZero() {
			conditions = append(conditions, "DATE(timestamp) <= ?")
			queryParams = append(queryParams, endDate.Format("2006-01-02"))
		}

		if len(conditions) > 0 {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}

		// Execute the query
		mu.Lock()
		rows, err := dataBase.Query(query, queryParams...) // querys the database and places it in rows and also used for err
		mu.Unlock()
		if err != nil {
			log.Printf("Database query failed: %v", err)
			http.Error(w, "Database query failed", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Collect results
		var results []Greeting
		for rows.Next() {
			var g Greeting
			err := rows.Scan(&g.FirstName, &g.LastName, &g.Message, &g.Timestamp)
			if err != nil {
				log.Printf("Failed to scan row: %v", err)
				http.Error(w, "Failed to process results", http.StatusInternalServerError)
				return
			}
			results = append(results, g) // appends the scanned greeting to the slice results
		}

		// Check for errors after iteration
		if err = rows.Err(); err != nil {
			log.Printf("Error iterating over rows: %v", err)
			http.Error(w, "Error processing results", http.StatusInternalServerError)
			return
		}

		// Return results as JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // this sends status code success
		err = json.NewEncoder(w).Encode(results)
		if err != nil {
			log.Printf("Failed to encode results to JSON: %v", err)
			http.Error(w, "Failed to encode results", http.StatusInternalServerError)
			return
		}

		log.Printf("Search successful, returned %d results.", len(results)) // this logs the amount results
	})

	// Start the HTTP server
	log.Println("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
