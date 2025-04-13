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

	query := "SELECT COUNT(*) FROM users WHERE username = $1 AND password_hash = $2"
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

	// Example: Check credentials in the database (replace with your own logic)
	query := "INSERT INTO users (username, password_hash) VALUES ($1, $2)"
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
