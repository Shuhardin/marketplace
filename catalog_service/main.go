// @title Catalog Service API
// @version 1.0
// @description API для управления товарами
// @host localhost
// @BasePath /api/catalog
package main

import (
	"log"
	"net/http"
	"os"
	"sync"

	_ "catalog_service/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

type Product struct {
	ID    string  `json:"id"`
	Name  string  `json:"name" binding:"required"`
	Price float64 `json:"price" binding:"required"`
}

var products = make(map[string]Product)
var productMutex sync.Mutex

var instanceID = os.Getenv("INSTANCE_NAME")

// GetProducts godoc
// @Summary Получить список товаров
// @Description Возвращает список всех товаров
// @Tags catalog
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /products [get]
func getProducts(c *gin.Context) { //получить список товаров
	productMutex.Lock()
	defer productMutex.Unlock()

	list := make([]Product, 0, len(products))
	for _, p := range products {
		list = append(list, p)
	}

	c.JSON(http.StatusOK, gin.H{
		"instance_id": instanceID,
		"products":    list,
		"count":       len(list),
	})
}

// GetProduct godoc
// @Summary Получить товар по ID
// @Description Возвращает один товар
// @Tags catalog
// @Produce json
// @Param id path string true "ID товара"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /products/{id} [get]
func getProduct(c *gin.Context) { //получить один товар
	id := c.Param("id")

	productMutex.Lock()
	product, exists := products[id]
	defer productMutex.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Товар не найден"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"instance_id": instanceID,
		"product":     product,
	})
}

// CreateProduct godoc
// @Summary Создать товар
// @Description Добавляет новый товар
// @Tags catalog
// @Accept json
// @Produce json
// @Param request body Product true "Данные товара"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /products [post]
func createProduct(c *gin.Context) {
	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name и price обязательны"})
		return
	}

	product.ID = uuid.New().String()

	productMutex.Lock()
	products[product.ID] = product
	productMutex.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"instance_id": instanceID,
		"message":     "Товар создан",
		"product":     product,
	})
}

// UpdateProduct godoc
// @Summary Обновить товар
// @Description Обновляет существующий товар
// @Tags catalog
// @Accept json
// @Produce json
// @Param id path string true "ID товара"
// @Param request body Product true "Данные товара"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /products/{id} [put]
func updateProduct(c *gin.Context) {
	id := c.Param("id")

	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name и price обязательны"})
		return
	}

	productMutex.Lock()
	defer productMutex.Unlock()

	if _, exists := products[id]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Товар не найден"})
		return
	}

	product.ID = id
	products[id] = product

	c.JSON(http.StatusOK, gin.H{
		"instance_id": instanceID,
		"message":     "Товар обновлен",
		"product":     product,
	})
}

// DeleteProduct godoc
// @Summary Удалить товар
// @Description Удаляет товар по ID
// @Tags catalog
// @Produce json
// @Param id path string true "ID товара"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /products/{id} [delete]
func deleteProduct(c *gin.Context) {
	id := c.Param("id")

	productMutex.Lock()
	defer productMutex.Unlock()

	if _, exists := products[id]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Товар не найден"})
		return
	}

	delete(products, id)

	c.JSON(http.StatusOK, gin.H{
		"instance_id": instanceID,
		"message":     "Товар удален",
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
		name = "catalog-local"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	registration := &api.AgentServiceRegistration{
		ID:      name,
		Name:    "catalog",
		Address: name,
		Port:    8082,
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

	r.GET("/products", getProducts)
	r.GET("/products/:id", getProduct)
	r.POST("/products", createProduct)
	r.PUT("/products/:id", updateProduct)
	r.DELETE("/products/:id", deleteProduct)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	r.GET("/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.Run(":" + port)
}
