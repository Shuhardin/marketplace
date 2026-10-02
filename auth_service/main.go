package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/consul/api"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte("secret-key")

var users = make(map[string]string)
var usersMutex sync.Mutex

type AuthRequest struct {
	Username string `json: "username" binding:"required"`
	Password string `json: "password" binding:"required"`
}

func register(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username и password обязательны"})
		return
	}

	usersMutex.Lock()
	defer usersMutex.Unlock()

	if _, exists := users[req.Username]; exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Такой пользователь уже существует"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка хеширования пароля"})
		return
	}

	users[req.Username] = string(hashedPassword)
	c.JSON(http.StatusCreated, gin.H{"message": "Пользователь создан"})
}

func login(c *gin.Context) {
	var req AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username и password обязательны"})
		return
	}

	usersMutex.Lock()
	hashedPassword, exists := users[req.Username]
	usersMutex.Unlock()

	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный логин или пароль"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неправильный пароль"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": req.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Ошибка создания токена"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
			c.Abort()
			return
		}

		tokenString := ""
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный формат токена"})
			c.Abort()
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Невалидный токен"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			c.Set("username", claims["username"])
		}

		c.Next()
	}
}

func me(c *gin.Context) {
	username, _ := c.Get("username")
	c.JSON(http.StatusOK, gin.H{
		"username": username,
		"message":  "Это защищенный эндпоинт",
	})
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func registerInConsul() {
	consulAddr := os.Getenv("CONSUL_ADDR")
	if consulAddr == "" {
		consulAddr = "localhost:8500"
	}
	config := api.DefaultConfig()
	config.Address = consulAddr

	client, err := api.NewClient(config)
	if err != nil {
		log.Println("Не удалось подключиться к Consul:", err)
		return
	}

	name := os.Getenv("INSTANCE_NAME")
	if name == "" {
		name = "auth-local"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	registration := &api.AgentServiceRegistration{
		ID:      name,
		Name:    "auth",
		Address: name,
		Port:    8081,
		Check: &api.AgentServiceCheck{
			HTTP:     "http://" + name + ":" + port + "/health",
			Interval: "10s",
			Timeout:  "5s",
		},
	}

	err = client.Agent().ServiceRegister(registration)
	if err != nil {
		log.Println("Ошибка регистрации в Consul:", err)
		return
	}

	log.Println("Зарегестрирован в Consul как", name)

}

func main() {
	registerInConsul()

	r := gin.Default()

	r.GET("/health", healthCheck)

	r.POST("/register", register)
	r.POST("/login", login)

	protected := r.Group("/")
	protected.Use(authMiddleware())
	protected.GET("/me", me)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	r.Run(":" + port)
}
