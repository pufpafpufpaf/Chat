package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

var db *sql.DB // Database connection variable

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // Allow all connections
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)

// Define the message structure
type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func initDB() {
	var err error
	connStr := "user=postgres password=puf781paf586puf963paf dbname=smt sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error connecting to the database: %v", err)
	}

	// Verify the connection
	err = db.Ping()
	if err != nil {
		log.Fatalf("Error verifying connection to the database: %v", err)
	}

	fmt.Println("Connected to the PostgreSQL database!")
}

func saveMessageToDB(msg Message) {
	query := "INSERT INTO messages (username, message) VALUES ($1, $2)"
	_, err := db.Exec(query, msg.Username, msg.Message)
	if err != nil {
		log.Printf("Error saving message to database: %v", err)
	}
}

func getLastMessages() ([]Message, error) {
	query := "SELECT username, message FROM messages" //ORDER BY id LIMIT 50"
	rows, err := db.Query(query)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message

	for rows.Next() {

		var msg Message

		if err := rows.Scan(&msg.Username, &msg.Message); err != nil {
			return nil, err
		}

		messages = append(messages, msg)
	}

	return messages, nil
}
func handleConnections(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()

	clients[conn] = true

	lastMessages, err := getLastMessages()
	if err != nil {
		fmt.Println("Error fetching last messages:", err)
		return
	}
	for _, msg := range lastMessages {
		if err := conn.WriteJSON(msg); err != nil {
			fmt.Println("Error sending last messages:", err)
			conn.Close()
			delete(clients, conn)
			return
		}
	}

	for {
		var msg Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			delete(clients, conn)
			break
		}

		// Save the message to the database
		saveMessageToDB(msg)

		broadcast <- msg
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func main() {
	initDB()
	defer db.Close()

	go handleMessages()

	r := gin.Default()
	r.Static("/static", "./static") // Serve frontend from 'static' folder

	r.GET("/", func(c *gin.Context) {
		c.File("./static/login.html") // Serve the main HTML file
	})

	r.POST("/login", func(c *gin.Context) {
		var credentials struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := db.Ping(); err != nil {
			log.Fatalf("Error verifying connection to the database: %v", err)
		}
		// Bind JSON data from the request body to the credentials struct
		if err := c.ShouldBindJSON(&credentials); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
			return
		}

		// Example: Validate credentials (you can replace this with your own logic)
		if credentials.Username == "" || credentials.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
			return
		}

		// Example: Check credentials in the database (replace with your own logic)
		query := "SELECT COUNT(*) FROM users WHERE name = $1 AND password_hash = $2"
		var count int
		err := db.QueryRow(query, credentials.Username, credentials.Password).Scan(&count)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		if count == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}

		// Respond with success
		c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
	})

	r.GET("/signup", func(c *gin.Context) {
		c.File("./static/signup.html")

	})

	r.GET("/chat", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	r.GET("/ws", handleConnections)

	fmt.Println("Server running on http://localhost:8080")
	r.Run("0.0.0.0:8080")
}
