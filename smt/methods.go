package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
	_ "github.com/lib/pq"
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

	// Create a new chat in the chats table
	var chatID int
	query = "INSERT INTO chats (name) VALUES ($1) RETURNING chat_id"
	err = db.QueryRow(query, fmt.Sprintf("Chat between %s and %s", currentUsername, request.Username)).Scan(&chatID)
	if err != nil {
		log.Printf("Error creating chat: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chat"})
		return
	}

	// Add rows in the chat_users table for both users
	query = "INSERT INTO chat_users (chat_id, user_id) VALUES ($1, $2)"
	_, err = db.Exec(query, chatID, currentUserID)
	if err != nil {
		log.Printf("Error adding current user to chat: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to chat"})
		return
	}

	_, err = db.Exec(query, chatID, senderUserID)
	if err != nil {
		log.Printf("Error adding sender user to chat: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user to chat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Friend request accepted and chat created"})
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

// GetFriendsWithChats retrieves the list of friends and their associated chat IDs for the logged-in user.
func GetFriendsWithChats(db *sql.DB, c *gin.Context) {
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

	// Query the database for friends and their associated chat IDs
	query := `
		SELECT u.username, c.chat_id
		FROM friends f
		JOIN users u ON (f.senduser_id = u.id AND f.recvuser_id = $1) OR (f.recvuser_id = u.id AND f.senduser_id = $1)
		JOIN chats c ON c.chat_id = (
			SELECT chat_id
			FROM chat_users
			WHERE user_id IN (f.senduser_id, f.recvuser_id)
			GROUP BY chat_id
			HAVING COUNT(DISTINCT user_id) = 2
		)
		WHERE f.accepted = true
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		log.Printf("Error fetching friends with chats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch friends"})
		return
	}
	defer rows.Close()

	// Collect the friends and their chat IDs
	var friends []map[string]interface{}
	for rows.Next() {
		var friendUsername string
		var chatID int
		if err := rows.Scan(&friendUsername, &chatID); err != nil {
			log.Printf("Error scanning friend row: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process friends"})
			return
		}
		friends = append(friends, map[string]interface{}{
			"username": friendUsername,
			"chat_id":  chatID,
		})
	}

	// Respond with the list of friends and their chat IDs
	c.JSON(http.StatusOK, gin.H{"friends": friends})
}

// GetChatMessages retrieves messages for a specific chat.
func GetChatMessages(db *sql.DB, c *gin.Context) {
	chatID := c.Query("chat_id")

	// Query the database for messages in the specified chat
	query := `
		SELECT u.username, m.message
		FROM messages m
		JOIN users u ON m.id_writer = u.id
		WHERE m.chat_recv_id = $1
		ORDER BY m.time
	`
	rows, err := db.Query(query, chatID)
	if err != nil {
		log.Printf("Error fetching chat messages: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch messages"})
		return
	}
	defer rows.Close()

	// Collect the messages
	var messages []map[string]string
	for rows.Next() {
		var username, message string
		if err := rows.Scan(&username, &message); err != nil {
			log.Printf("Error scanning message row: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process messages"})
			return
		}
		messages = append(messages, map[string]string{
			"username": username,
			"message":  message,
		})
	}

	// Respond with the list of messages
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// CreateGroupChat handles group chat creation in the database.
