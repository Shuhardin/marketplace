package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
)

/////////СТРУКТУРЫ ДАННЫХ////////

type Product struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type OrderItem struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Product   Product `json:"product"`
}

type Order struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	CreatedAt string      `json:"created_at"`
	Items     []OrderItem `json:"items"`
}

// ////////////ЗАПРОСЫ К REST////////////
// Подтягиваем данные о товарах, используя ID заказов, полученные от резольвера ordersByUserId

// Идем в Order за заказами
func fetchOrders(userID string) ([]Order, error) {
	url := "http://order:8084/api/orders?userId=" + userID
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Orders []Order `json:"orders"`
	}
	json.Unmarshal(body, &result)
	return result.Orders, nil
}

// Идем в Catalog за товаром
func fetchProduct(productID string) (Product, error) {
	url := "http://nginx/api/catalog/products/" + productID
	resp, err := http.Get(url)
	if err != nil {
		return Product{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Product Product `json:"product"`
	}
	json.Unmarshal(body, &result)
	return result.Product, nil
}

var productCache = make(map[string]Product)
var cacheMutex sync.Mutex

func fetchProductCached(productID string) (Product, error) { //кэширование товаров в productCache
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if p, ok := productCache[productID]; ok {
		log.Println("Товар есть в кэше:", productID)
		return p, nil
	}

	log.Println("Товар из каталога:", productID)
	product, err := fetchProduct(productID)
	if err != nil {
		log.Println("Нет такого товара")
		return Product{}, err
	}

	productCache[productID] = product
	return product, nil
}

///////////GRAPHQL-СХЕМА/////////////

// Тип Product
var productType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "Product",
	Description: "Товар из каталога",
	Fields: graphql.Fields{
		"id":    &graphql.Field{Type: graphql.ID, Description: "ID товара"},
		"name":  &graphql.Field{Type: graphql.String, Description: "Название товара"},
		"price": &graphql.Field{Type: graphql.Float, Description: "Цена товара"},
	},
})

// Тип OrderItem
var orderItemType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "OrderItem",
	Description: "Элемент заказа",
	Fields: graphql.Fields{
		"quantity": &graphql.Field{Type: graphql.Int, Description: "Количество"},
		"product": &graphql.Field{
			Type:        productType,
			Description: "Товар",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				item := p.Source.(OrderItem)
				return fetchProductCached(item.ProductID) //агрегация
			},
		},
	},
})

// Тип Order
var orderType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "Order",
	Description: "Заказ пользователя",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.ID, Description: "ID заказа"},
		"userId":    &graphql.Field{Type: graphql.ID, Description: "ID пользователя"},
		"createdAt": &graphql.Field{Type: graphql.String, Description: "Дата создания"},
		"items": &graphql.Field{
			Type:        graphql.NewList(orderItemType),
			Description: "Товары в заказе",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				order := p.Source.(Order)
				return order.Items, nil
			},
		},
	},
})

// Query - точка входа
var queryType = graphql.NewObject(graphql.ObjectConfig{
	Name: "Query",
	Fields: graphql.Fields{
		"ordersByUserId": &graphql.Field{
			Type:        graphql.NewList(orderType),
			Description: "Получить все заказы пользователя с деталями товаров",
			Args: graphql.FieldConfigArgument{
				"userId": &graphql.ArgumentConfig{
					Type:        graphql.NewNonNull(graphql.ID),
					Description: "ID пользователя",
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				userID := p.Args["userId"].(string)
				return fetchOrders(userID) //идём в Order
			},
		},
	},
})

func main() {
	// Создаём схему
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Создаём handler
	h := handler.New(&handler.Config{
		Schema:     &schema,
		Pretty:     true,
		Playground: true,
	})

	r := gin.Default()
	r.POST("/graphql", func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	})
	r.GET("/graphql", func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	})
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	r.Run(":" + port)
}
