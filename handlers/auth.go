package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SleepyXm/SynapseRGo/structs"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func Signup(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req structs.UserCreate
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// Check username
		var exists string
		err := db.QueryRow("SELECT id FROM users WHERE username = $1", req.Username).Scan(&exists)
		if err != sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username taken, try another."})
			return
		}

		// Check email
		//err = db.QueryRow("SELECT id FROM users WHERE email = $1", req.Email).Scan(&exists)
		//if err != sql.ErrNoRows {
		//	c.JSON(http.StatusBadRequest, gin.H{"error": "Email already registered"})
		//	return
		//}

		hashed, err := hashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
			return
		}

		_, err = db.Exec(
			"INSERT INTO users (id, username, password, created_at) VALUES ($1, $2, $3, $4, NOW())",
			uuid.New().String(), req.Username, hashed,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User created successfully"})
	}
}

func Login(db *sql.DB, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req structs.UserLogin
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var id, passwordHash string
		err := db.QueryRow("SELECT id, password FROM users WHERE username = $1", req.Username).Scan(&id, &passwordHash)

		// timing safe — always verify even if user not found
		if err == sql.ErrNoRows {
			verifyPassword(req.Password, DUMMY_PASSWORD_HASH)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username or Password Incorrect"})
			return
		}

		if !verifyPassword(req.Password, passwordHash) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username or Password Incorrect"})
			return
		}

		claims := jwt.MapClaims{
			"sub": id,
			"exp": time.Now().Add(24 * 7 * time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(jwtSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
			return
		}

		c.SetCookie(
			"access_token",
			"Bearer "+tokenString,
			60*60*24*7,
			"/",
			"",
			true,
			true,
		)

		c.JSON(http.StatusOK, gin.H{"message": "Login successful", "token": tokenString})
	}
}

func Me(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		var username string
		var hfTokens []byte
		err := db.QueryRow("SELECT username, hf_tokens FROM users WHERE id = $1::uuid", userID).Scan(&username, &hfTokens)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		var tokenList []string
		if hfTokens != nil {
			json.Unmarshal(hfTokens, &tokenList)
		}

		c.JSON(http.StatusOK, gin.H{
			"username": username,
			"hf_token": tokenList,
		})
	}
}

func Logout(c *gin.Context) {
	c.SetCookie("access_token", "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("pgx", os.Getenv("DATABASE"))
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("DB not reachable:", err)
	}

	log.Println("DB connected")
}

func Hi(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Auth router is working!"})
}
