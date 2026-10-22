package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func main() {
	connectRabbitMQ()

	//подписка на очередь OrderCreated
	subscribeToQueue("order_created", func(body []byte) {
		var event OrderCreatedEvent
		if err := json.Unmarshal(body, &event); err != nil {
			log.Println("Ошибка парсинга json OrderCreated:", err)
			return
		}

		log.Printf("Обработка заказа %s на сумму %.2f\n", event.OrderID, event.TotalAmount)

		//имитация оплаты
		//если сумма <= 0, то будет ошибка
		//иначе 70% вероятность успеха
		if event.TotalAmount <= 0 {
			log.Printf("Оплата заказа %s не прошла, сумма <= 0 \n", event.OrderID)
			publishEvent("payment_failed", PaymentFailedEvent{
				OrderID: event.OrderID,
				Reason:  "Сумма заказа <= 0",
			})
			return
		}

		if rand.Intn(100) < 70 {
			log.Printf("Оплата заказа %s прошла успешно:", event.OrderID)
			publishEvent("payment_completed", PaymentCompletedEvent{
				OrderID:       event.OrderID,
				TransactionID: uuid.New().String(),
			})
		} else {
			log.Printf("Оплата заказа %s не прошла:", event.OrderID)
			publishEvent("payment_failed", PaymentFailedEvent{
				OrderID: event.OrderID,
				Reason:  "Оплата не прошла, попробуйте снова",
			})
		}
	})

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
	}

	r.Run(":" + port)
}
