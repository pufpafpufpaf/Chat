package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

// Login handles user authentication by verifying credentials against the database.
func Login(db *sql.DB, c *gin.Context) (bool, error) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// Ensure the database connection is active
	if err := db.Ping(); err != nil {
		log.Fatalf("Error verifying connection to the database: %v", err)
	}

	// Parse the JSON request body into the credentials struct
	if err := c.ShouldBindJSON(&credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
		return false, err
	}

	// Validate that both username and password are provided
	if credentials.Username == "" || credentials.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return false, nil
	}

	// Query the database to check if the username and password match
	query := "SELECT COUNT(*) FROM users WHERE username = $1 AND password = $2"
	var count int
	err := db.QueryRow(query, credentials.Username, credentials.Password).Scan(&count)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return false, err
	}

	// If no matching user is found, return an unauthorized error
	if count == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return false, nil
	}

	// Store the username in the session for future requests
	session := c.MustGet("session").(*sessions.Session)
	session.Values["username"] = credentials.Username
	session.Save(c.Request, c.Writer)

	// Respond with a success message
	c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
	return true, nil
}

// Signup handles user registration by adding new users to the database.
func Signup(db *sql.DB, c *gin.Context) (bool, error) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// Ensure the database connection is active
	if err := db.Ping(); err != nil {
		log.Fatalf("Error verifying connection to the database: %v", err)
	}

	// Parse the JSON request body into the credentials struct
	if err := c.ShouldBindJSON(&credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
		return false, err
	}

	// Validate that both username and password are provided
	if credentials.Username == "" || credentials.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return false, nil
	}

	// Insert the new user into the database
	query := "INSERT INTO users (username, password) VALUES ($1, $2)"
	_, err := db.Exec(query, credentials.Username, credentials.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return false, err
	}

	// Store the username in the session for future requests
	session := c.MustGet("session").(*sessions.Session)
	session.Values["username"] = credentials.Username
	session.Save(c.Request, c.Writer)

	// Respond with a success message
	c.JSON(http.StatusOK, gin.H{"message": "SignUp successful"})
	return true, nil
}

// AuthRequired is a middleware that ensures the user is authenticated before accessing certain routes.
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := c.MustGet("session").(*sessions.Session)
		username := session.Values["username"]

		// If no username is found in the session, return an unauthorized error
		if username == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Proceed to the next handler
		c.Next()
	}
}

// SendFriendRequest handles sending a friend request by inserting it into the friends table.
func SendFriendRequest(db *sql.DB, c *gin.Context) {
	var recvusername struct {
		Username string `json:"username"`
	}

	// Retrieve the sender's username from the session
	session := c.MustGet("session").(*sessions.Session)
	sendusername := session.Values["username"].(string)

	// Ensure the database connection is active
	if err := db.Ping(); err != nil {
		log.Fatalf("Error verifying connection to the database: %v", err)
	}

	// Parse the JSON request body into the recvusername struct
	if err := c.ShouldBindJSON(&recvusername); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
		return
	}

	// Validate that the receiver's username is provided
	if recvusername.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	// Fetch the IDs of the sender and receiver from the users table
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

	// Check if a friend request already exists or if they are already friends
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 
			FROM friends 
			WHERE (senduser_id = $1 AND recvuser_id = $2) 
			   OR (senduser_id = $2 AND recvuser_id = $1)
		)
	`
	err = db.QueryRow(query, sendUserID, recvUserID).Scan(&exists)
	if err != nil {
		log.Printf("Error checking existing friend request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check friend request"})
		return
	}

	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Friend request already exists or users are already friends"})
		return
	}

	// Insert the friend request into the friends table
	query = "INSERT INTO friends (senduser_id, recvuser_id) VALUES ($1, $2)"
	_, err = db.Exec(query, sendUserID, recvUserID)
	if err != nil {
		log.Printf("Error inserting friend request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send friend request"})
		return
	}

	// Respond with a success message
	c.JSON(http.StatusOK, gin.H{"message": "Friend request sent successfully"})
}

// GetFriendRequests retrieves pending friend requests for the logged-in user.
func GetFriendRequests(db *sql.DB, c *gin.Context) {
	// Retrieve the logged-in user's username from the session
	session := c.MustGet("session").(*sessions.Session)
	username := session.Values["username"].(string)

	// Fetch the user ID of the logged-in user
	var userID int
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", username).Scan(&userID)
	if err != nil {
		log.Printf("Error fetching user ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user ID"})
		return
	}

	// Query the database for pending friend requests
	query := `
		SELECT u.username 
		FROM friends f 
		JOIN users u ON f.senduser_id = u.id 
		WHERE f.recvuser_id = $1 AND f.accepted = false
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		log.Printf("Error fetching friend requests: %v", err)
		return
	}
	defer rows.Close()

	// Collect the usernames of users who sent friend requests
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

	// Respond with the list of pending friend requests
	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

// AcceptFriendRequest updates the "accepted" field to true for a friend request.
func AcceptFriendRequest(db *sql.DB, c *gin.Context) {
	var request struct {
		Username string `json:"username"`
	}

	// Retrieve the logged-in user's username from the session
	session := c.MustGet("session").(*sessions.Session)
	currentUsername := session.Values["username"].(string)

	// Parse the JSON request body
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
		return
	}

	// Fetch the IDs of the current user and the sender
	var currentUserID, senderUserID int
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", currentUsername).Scan(&currentUserID)
	if err != nil {
		log.Printf("Error fetching current user ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user ID"})
		return
	}

	err = db.QueryRow("SELECT id FROM users WHERE username = $1", request.Username).Scan(&senderUserID)
	if err != nil {
		log.Printf("Error fetching sender user ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sender ID"})
		return
	}

	// Update the "accepted" field to true in the friends table
	query := "UPDATE friends SET accepted = true WHERE senduser_id = $1 AND recvuser_id = $2"
	_, err = db.Exec(query, senderUserID, currentUserID)
	if err != nil {
		log.Printf("Error accepting friend request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to accept friend request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend request accepted"})
}

// DeleteFriendRequest deletes a friend request from the database.
func DeleteFriendRequest(db *sql.DB, c *gin.Context) {
	var request struct {
		Username string `json:"username"`
	}

	// Retrieve the logged-in user's username from the session
	session := c.MustGet("session").(*sessions.Session)
	currentUsername := session.Values["username"].(string)

	// Parse the JSON request body
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON data"})
		return
	}

	// Fetch the IDs of the current user and the sender
	var currentUserID, senderUserID int
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", currentUsername).Scan(&currentUserID)
	if err != nil {
		log.Printf("Error fetching current user ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user ID"})
		return
	}

	err = db.QueryRow("SELECT id FROM users WHERE username = $1", request.Username).Scan(&senderUserID)
	if err != nil {
		log.Printf("Error fetching sender user ID: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch sender ID"})
		return
	}

	// Delete the friend request from the friends table
	query := "DELETE FROM friends WHERE senduser_id = $1 AND recvuser_id = $2"
	_, err = db.Exec(query, senderUserID, currentUserID)
	if err != nil {
		log.Printf("Error deleting friend request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete friend request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend request deleted"})
}
