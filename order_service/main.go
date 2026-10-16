package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

type OrderItem struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required"`
}

type Order struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id" binding:"required"`
	CreatedAt string      `json:"created_at"`
	Items     []OrderItem `json:"items" binding:"required"`
}

var orders = make(map[string]Order)
var ordersMutex sync.Mutex

var instanceID = os.Getenv("INSTANCE_NAME")

func getOrders(c *gin.Context) { //заказы пользователя
	var userID = c.Query("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "обязателен userID"})
		return
	}

	ordersMutex.Lock()
	defer ordersMutex.Unlock()

	var list = make([]Order, 0)
	for _, o := range orders {
		if userID == o.UserID {
			list = append(list, o)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"instance_id": instanceID,
		"orders":      list,
		"count":       len(list),
	})
}

func getOrder(c *gin.Context) { //заказ по ID
	id := c.Param("id")

	ordersMutex.Lock()
	defer ordersMutex.Unlock()

	order, exists := orders[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "нет такого заказа"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"instance_id": instanceID,
		"order":       order,
	})
}

func createOrder(c *gin.Context) {
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id и items обязательны"})
		return
	}

	order.ID = uuid.New().String()
	order.CreatedAt = time.Now().Format(time.RFC3339)

	ordersMutex.Lock()
	orders[order.ID] = order
	ordersMutex.Unlock()

	c.JSON(http.StatusCreated, gin.H{
		"instance_id": instanceID,
		"message":     "Заказ создан",
		"order":       order,
	})
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "OK"})
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
		name = "order-local"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	registration := &api.AgentServiceRegistration{
		ID:      name,
		Name:    "order",
		Address: name,
		Port:    8084,
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

	log.Println("Зарегистрирован в Consul как", name)
}

func main() {
	registerInConsul()

	r := gin.Default()

	r.GET("/health", healthCheck)
	r.GET("/api/orders", getOrders)
	r.GET("/api/orders/:id", getOrder)
	r.POST("/api/orders", createOrder)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	r.Run(":" + port)
}
