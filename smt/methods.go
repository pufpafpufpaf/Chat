package main

import (
	"database/sql"
	"fmt"
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

	// Create a new chat in the chats table
	var chatID int
	query = "INSERT INTO chats (name) VALUES ($1) RETURNING chat_id"
	err = db.QueryRow(query, fmt.Sprintf("%s and %s", currentUsername, request.Username)).Scan(&chatID)
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

	// Query for both individual chats and group chats
	query := `
		SELECT DISTINCT 
			CASE 
				WHEN c.name LIKE 'Chat between%' THEN 
					COALESCE(u.username, c.name)
				ELSE c.name 
			END as display_name,
			c.chat_id
		FROM chat_users cu
		JOIN chats c ON cu.chat_id = c.chat_id
		LEFT JOIN chat_users cu2 ON c.chat_id = cu2.chat_id AND cu2.user_id != $1
		LEFT JOIN users u ON cu2.user_id = u.id
		WHERE cu.user_id = $1
		ORDER BY c.chat_id
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		log.Printf("Error fetching chats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chats"})
		return
	}
	defer rows.Close()

	// Collect the friends and their chat IDs
	var friends []map[string]interface{}
	for rows.Next() {
		var name string
		var chatID int
		if err := rows.Scan(&name, &chatID); err != nil {
			log.Printf("Error scanning chat row: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process chats"})
			return
		}
		friends = append(friends, map[string]interface{}{
			"username": name,
			"chat_id":  chatID,
		})
	}

	// Respond with the list of friends and their chat IDs
	c.JSON(http.StatusOK, gin.H{"friends": friends})
}

// GetChatMessages retrieves messages for a specific chat.
func GetChatMessages(db *sql.DB, c *gin.Context) {
	chatID := c.Query("chat_id")

	// If no chat ID is provided, it's All Chat
	if chatID == "" || chatID == "0" {
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
		c.JSON(http.StatusOK, gin.H{
			"messages": messages,
			"chatName": "All Chat",
		})
		return
	}

	// Get the chat name
	var chatName string
	err := db.QueryRow("SELECT name FROM chats WHERE chat_id = $1", chatID).Scan(&chatName)
	if err != nil {
		log.Printf("Error fetching chat name: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chat name"})
		return
	}

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
	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"chatName": chatName,
	})
}

// CreateGroupChat creates a new group chat with the specified users
func CreateGroupChat(db *sql.DB, c *gin.Context) {
	var request struct {
		Name    string   `json:"name"`
		Friends []string `json:"friends"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Get the current user's ID
	session := c.MustGet("session").(*sessions.Session)
	username := session.Values["username"].(string)
	var currentUserID int
	err := db.QueryRow("SELECT id FROM users WHERE username = $1", username).Scan(&currentUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user ID"})
		return
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Create the chat
	var chatID int
	err = tx.QueryRow("INSERT INTO chats (name) VALUES ($1) RETURNING chat_id", request.Name).Scan(&chatID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create chat"})
		return
	}

	// Add the current user to the chat
	_, err = tx.Exec("INSERT INTO chat_users (chat_id, user_id) VALUES ($1, $2)", chatID, currentUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add current user"})
		return
	}

	// Add all selected friends to the chat
	for _, friend := range request.Friends {
		var friendID int
		err := tx.QueryRow("SELECT id FROM users WHERE username = $1", friend).Scan(&friendID)
		if err != nil {
			continue // Skip if friend not found
		}
		_, err = tx.Exec("INSERT INTO chat_users (chat_id, user_id) VALUES ($1, $2)", chatID, friendID)
		if err != nil {
			continue // Skip if insert fails
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Group chat created", "chat_id": chatID})
}
