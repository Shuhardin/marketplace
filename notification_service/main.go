package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/consul/api"
)

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
		name = "notification"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8087"
	}

	registration := &api.AgentServiceRegistration{
		ID:      name,
		Name:    "notification",
		Address: name,
		Port:    8087,
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
	connectRabbitMQ()

	subscribeToQueue("payment_completed", func(body []byte) {
		var event PaymentCompletedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			log.Println("Ошибка парсинга PaymentCompleted", err)
			return
		}

		log.Printf("Успех! Оплата по заказу %s прошла, транзакция %s \n", event.OrderID, event.TransactionID)
		log.Println("Отправлено уведомление клиенту: заказ успешно оплачен")
	})

	subscribeToQueue("order_cancelled", func(body []byte) {
		var event OrderCancelledEvent
		if err := json.Unmarshal(body, &event); err != nil {
			log.Println("Ошибка парсинга OrderCancelled", err)
			return
		}

		log.Printf("Отмена! Заказ %s не оплачен \n", event.OrderID)
		log.Println("Отправлено уведомление клиенту: заказ отменен")
	})

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8087"
	}

	r.Run(":" + port)
}
