package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

func Login(db *sql.DB, c *gin.Context) (bool, error) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Error verifying connection to the database: %v", err)
	}

	if err := c.ShouldBindJSON(&credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
		return false, err
	}

	if credentials.Username == "" || credentials.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return false, nil
	}

	query := "SELECT COUNT(*) FROM users WHERE username = $1 AND password = $2"
	var count int
	err := db.QueryRow(query, credentials.Username, credentials.Password).Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return false, err
	}

	if count == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return false, nil
	}

	session := c.MustGet("session").(*sessions.Session)
	session.Values["username"] = credentials.Username // Store username in session
	session.Save(c.Request, c.Writer)

	// Respond with success
	c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
	return true, nil
}

func Signup(db *sql.DB, c *gin.Context) (bool, error) {
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
		return false, err
	}

	// Example: Validate credentials (you can replace this with your own logic)
	if credentials.Username == "" || credentials.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return false, nil
	}

	query := "INSERT INTO users (username, password) VALUES ($1, $2)"
	_, err := db.Exec(query, credentials.Username, credentials.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return false, err
	}

	session := c.MustGet("session").(*sessions.Session)
	session.Values["username"] = credentials.Username // Store username in session
	session.Save(c.Request, c.Writer)

	// Respond with success
	c.JSON(http.StatusOK, gin.H{"message": "SignUp successful"})
	return true, nil
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := c.MustGet("session").(*sessions.Session)
		username := session.Values["username"] // Retrieve username from session

		if username == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort() // Stop further processing
			return
		}

		// Pass the request to the next handler
		c.Next()
	}

}
func SendFriendRequest(db *sql.DB, c *gin.Context) {
	var recvusername struct {
		Username string `json:"username"`
	}
	session := c.MustGet("session").(*sessions.Session)
	sendusername := session.Values["username"].(string) // Ensure sendusername is a string

	if err := db.Ping(); err != nil {
		log.Fatalf("Error verifying connection to the database: %v", err)
	}

	if err := c.ShouldBindJSON(&recvusername); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
		return
	}

	// Log the received username for debugging
	log.Printf("Received friend request for username: %s", recvusername.Username)

	if recvusername.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	// Fetch the IDs of the sender and receiver
	var sendUserID, recvUserID int
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", sendusername).Scan(&sendUserID)
	if err != nil {
		log.Printf("Error fetching sender ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sender ID"})
		return
	}

	err = db.QueryRow("SELECT id FROM users WHERE username = $1", recvusername.Username).Scan(&recvUserID)
	if err != nil {
		log.Printf("Error fetching receiver ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch receiver ID"})
		return
	}

	// Insert the friend request into the friends table
	query := "INSERT INTO friends (senduser_id, recvuser_id) VALUES ($1, $2)"
	_, err = db.Exec(query, sendUserID, recvUserID)
	if err != nil {
		log.Printf("Error inserting friend request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send friend request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend request sent successfully"})
}

func GetFriendRequests(db *sql.DB, c *gin.Context) {
	session := c.MustGet("session").(*sessions.Session)
	username := session.Values["username"].(string) // Ensure username is a string

	// Fetch the user ID of the logged-in user
	var userID int
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", username).Scan(&userID)
	if err != nil {
		log.Printf("Error fetching user ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user ID"})
		return
	}

	// Fetch pending friend requests where the logged-in user is the receiver
	query := `
		SELECT u.username 
		FROM friends f 
		JOIN users u ON f.senduser_id = u.id 
		WHERE f.recvuser_id = $1 AND f.accepted = false
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		log.Printf("Error fetching friend requests: %v", err)
		//c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch friend requests"})
		return
	}
	defer rows.Close()

	var requests []string
	for rows.Next() {
		var senderUsername string
		if err := rows.Scan(&senderUsername); err != nil {
			log.Printf("Error scanning friend request row: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process friend requests"})
			return
		}
		requests = append(requests, senderUsername)
	}

	c.JSON(http.StatusOK, gin.H{"requests": requests})
}
