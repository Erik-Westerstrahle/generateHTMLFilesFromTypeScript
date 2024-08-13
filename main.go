package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
)

// Greeting represents a simple structure for a greeting message
type Greeting struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Message   string `json:"message"`
}

// Store greetings in a global slice
var greetings []Greeting
var mu sync.Mutex // to protect concurrent access to the greetings slice

// holds data that will be passed to HTML
type PageData struct {
	Title      string
	JavaScript template.JS
	Message    string
}

func main() {

	// Serve static files from the "static" directory
	//http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Read the compiled JavaScript code
	jsData, err := os.ReadFile("main.js")
	if err != nil {
		log.Fatalf("Failed to read JavaScript file: %v", err)
	}

	// Define the HTML template
	tmpl := template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <link rel="stylesheet" type="text/css" href="/static/styles.css"> <!-- Example static CSS file -->
</head>
<body>
    <h1>Check the console for the greeting!</h1>
    <form action="/greet" method="POST">
         <input type="text" name="first_name" placeholder="Enter your first name">
         <input type="text" name="last_name" placeholder="Enter your last name">
        <button type="submit">Greet Me</button>
    </form>

    {{if .Message}}
    <p>{{.Message}}</p>
    {{end}}

    <h2>Greeting List:</h2>
    <ul id="greetingList"></ul>

    <script>
        // Fetch the greetings from the backend and display them
        fetch('/greetings')
            .then(response => response.json())
            .then(data => {
                const greetingList = document.getElementById('greetingList');
                data.forEach(greeting => {
                    const li = document.createElement('li');
                    li.textContent = greeting.first_name + " " + greeting.last_name + ": " + greeting.message;
                    greetingList.appendChild(li);
                });
            })
            .catch(error => console.error('Error fetching greetings:', error));

        {{.JavaScript}}
    </script>
</body>
</html>
    `))

	// Handle the root path and render the template
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		pageData := PageData{
			Title:      "Go Generated Page",
			JavaScript: template.JS(jsData), // javascript code that is inluded
		}
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
				http.Error(w, "First and last names are required", http.StatusBadRequest)
				return
			}

			message := fmt.Sprintf("Thank you, %s %s! Your greeting has been recorded.", firstName, lastName)

			// Add the new greeting to the global greetings slice
			mu.Lock()

			// appends new elemt to greetings slice
			// creates greeting struct
			greetings = append(greetings, Greeting{
				FirstName: firstName, // assign vale firstName to value FirstName
				LastName:  lastName,
				Message:   fmt.Sprintf("Hello, %s %s!", firstName, lastName),
			})
			mu.Unlock()

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
		if r.Method == http.MethodGet {
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
		}
	})

	// Start the HTTP server
	log.Println("Server is running on http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
