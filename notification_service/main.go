package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
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
