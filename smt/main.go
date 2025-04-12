package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
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
	fmt.Println("Query executed:")
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
func handleConnections(c *gin.Context, username interface{}) {

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

		msg.Username = username.(string)
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

var store = sessions.NewCookieStore([]byte("sec~?>!HSC|I\"s$JPkmU-m|#~o~:L_z{\"[gF5pt^vckg:`vE<n7R6Hf;u6_[OMe5b5ret")) // Replace "secret" with a secure key

var r = gin.Default()

func main() {
	initDB()
	defer db.Close()

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600, // 1 hour
		HttpOnly: false,
	}

	// Replace Gin's session middleware with Gorilla's session handling
	r.Use(func(c *gin.Context) {
		session, _ := store.Get(c.Request, "mysession")
		c.Set("session", session)
		c.Next()
	})

	//r.Use(AuthRequired())
	go handleMessages()

	r.Static("/static", "./static") // Serve frontend from 'static' folder

	r.GET("/", func(c *gin.Context) {
		c.File("./static/login.html") // Serve the main HTML file
	})

	r.POST("/login", func(c *gin.Context) {
		Login(db, c)
	})

	r.GET("/signup", func(c *gin.Context) {
		c.File("./static/signup.html")
	})

	r.POST("/signup", func(c *gin.Context) {
		Signup(db, c)
	})

	r.GET("/chat", AuthRequired(), func(c *gin.Context) {
		session := c.MustGet("session").(*sessions.Session)
		username := session.Values["username"]

		if username == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		fmt.Println(username)

		c.File("./static/index.html")
	})

	r.GET("/ws", func(c *gin.Context) {
		session := c.MustGet("session").(*sessions.Session)
		username := session.Values["username"]
		fmt.Println(username)

		if username == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		fmt.Println("/ws good")

		handleConnections(c, username)
	}) // WebSocket endpoint

	fmt.Println("Server running on http://localhost:8080")
	r.Run("0.0.0.0:8080")
}
