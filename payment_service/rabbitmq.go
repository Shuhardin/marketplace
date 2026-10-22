package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var rabbitConn *amqp.Connection
var rabbitChannel *amqp.Channel

type OrderItem struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

type OrderCreatedEvent struct { //событие создание заказа
	OrderID     string      `json:"orderId"`
	UserID      string      `json:"userId"`
	Items       []OrderItem `json:"items"`
	TotalAmount float64     `json:"totalAmount"`
	CreatedAt   string      `json:"createdAt"`
}

type PaymentCompletedEvent struct { //успешно выполнена оплата
	OrderID       string `json:"orderId"`
	TransactionID string `json:"transactionId"`
}

type PaymentFailedEvent struct { //операция оплаты провалена
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

func connectRabbitMQ() { //подключение к RabbitMQ
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

	rabbitChannel, err = rabbitConn.Channel() //создание канала связи в туннеле Conn
	if err != nil {
		log.Fatal("Не удалось подключить канал", err)
	}

	//объявление очередей
	declareQueue("order_created")
	declareQueue("payment_completed")
	declareQueue("payment_failed")

	log.Println("Подключено к RabbitMQ")
}

// объявление очереди
// если очереди нет, то она создается
// а если она есть, то ничего не происходит
func declareQueue(name string) {
	_, err := rabbitChannel.QueueDeclare(
		name,  //имя очереди
		true,  //сохранение при перезапуске
		false, //auto-delete
		false, //exclusive доступа всем
		false, //no wait ждем подтверждения
		nil,   //доп параметры
	)
	if err != nil {
		log.Fatal("Ошибка объявления очереди:", err)
	}
}

func publishEvent(queueName string, event interface{}) {
	body, err := json.Marshal(event)
	if err != nil {
		log.Println("Ошибка сериализации", err)
		return
	}

	err = rabbitChannel.Publish(
		"",        //exchange
		queueName, //routing key имя очереди
		false,     //mandatory подтверждение
		false,     //immediate немедленная доставка
		amqp.Publishing{ //сообщение для отправки
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		log.Println("Ошибка публикации", err)
		return
	}

	log.Printf("Опубликовано в %s: %s\n", queueName, string(body))
}

func subscribeToQueue(queueName string, handler func([]byte)) {
	msgs, err := rabbitChannel.Consume( //метод канала
		queueName, //имя очереди
		"",        //consumer tag
		true,      //auto-ack
		false,     // exclucive
		false,     //no local
		false,     //no wait
		nil,       //args
	)
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
