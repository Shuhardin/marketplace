package main

import (
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var rabbitConn *amqp.Connection
var rabbitChannel *amqp.Channel

// Только те структуры, которые Notification читает
type PaymentCompletedEvent struct {
	OrderID       string `json:"orderId"`
	TransactionID string `json:"transactionId"`
}

type OrderCancelledEvent struct {
	OrderID string `json:"orderId"`
}

func connectRabbitMQ() {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	var err error
	for i := 0; i < 10; i++ {
		rabbitConn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Println("Попытка подключения к RabbitMQ", i+1)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatal("Не удалось подключиться к RabbitMQ", err)
	}

	rabbitChannel, err = rabbitConn.Channel()
	if err != nil {
		log.Fatal("Не удалось создать канал", err)
	}

	// Объявляем только те очереди, которые читаем
	declareQueue("payment_completed")
	declareQueue("order_cancelled")

	log.Println("Подключено к RabbitMQ")
}

func declareQueue(name string) {
	_, err := rabbitChannel.QueueDeclare(name, true, false, false, false, nil)
	if err != nil {
		log.Fatal("Ошибка объявления очереди:", err)
	}
}

// publishEvent нет
// Notification только читает

func subscribeToQueue(queueName string, handler func([]byte)) {
	msgs, err := rabbitChannel.Consume(queueName, "", true, false, false, false, nil)
	if err != nil {
		log.Fatal("Ошибка подписки:", err)
	}

	go func() {
		for msg := range msgs {
			log.Printf("Получено из %s: %s\n", queueName, string(msg.Body))
			handler(msg.Body)
		}
	}()
}
