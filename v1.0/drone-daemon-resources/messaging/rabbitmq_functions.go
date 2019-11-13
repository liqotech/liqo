package messaging

import (
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"os"
)

type RabbitMq struct {
	conn    *amqp.Connection
	ch      *amqp.Channel
	Q       amqp.Queue
	rbmqErr error
}

// SetConn receives a pointer to RabbitMq so it can modify it. (StayTheSame)
func (confRabbit *RabbitMq) SetConn(conn *amqp.Connection) {
	confRabbit.conn = conn
}

// SetCh receives a pointer to RabbitMq so it can modify it. (Mutate)
func (confRabbit *RabbitMq) SetCh(ch *amqp.Channel) {
	confRabbit.ch = ch
}

// SetCh receives a pointer to RabbitMq so it can modify it. (Mutate)
func (confRabbit *RabbitMq) SetErr(err error) {
	confRabbit.rbmqErr = err
}

// SetConn receives a copy of RabbitMq since it doesn't need to modify it.
func (confRabbit RabbitMq) Conn() *amqp.Connection {
	return confRabbit.conn
}

// SetCh receives a copy of RabbitMq since it doesn't need to modify it.
func (confRabbit RabbitMq) Ch() *amqp.Channel {
	return confRabbit.ch
}

func InitRabbitMq(queueName string, brokerAddress string, brokerPort string, virtualHost string, username string, password string) *RabbitMq {

	config := &RabbitMq{}

	config.Connect(brokerAddress, brokerPort, virtualHost, username, password)

	config.CreateChannel()

	config.DeclareQueue(queueName)

	return config
}

// Connect to RabbitMq
func (confRabbit *RabbitMq) Connect(brokerAddress string, brokerPort string, virtualHost string, username string, password string) {

	//print("amqp://" + username + ":" + password + "@rabbitmq-service:5672/")
	//conn, err := amqp.Dial("amqp://drone:drone@rabbitmq-service:5672/")
	conn, err := amqp.Dial("amqp://" + username + ":" + password + "@" + brokerAddress + ":" + brokerPort + "/" + virtualHost)
	failOnError(err, "Failed to connect to RabbitMQ")
	confRabbit.SetConn(conn)
	confRabbit.SetErr(err)
}

// Create Channel
func (confRabbit *RabbitMq) CreateChannel() {
	ch, err := confRabbit.Conn().Channel()
	failOnError(err, "Failed to open a channel")
	confRabbit.SetCh(ch)
	confRabbit.SetErr(err)
}

// Declare a Queue
func (confRabbit *RabbitMq) DeclareQueue(queueName string) {
	q, err := confRabbit.Ch().QueueDeclare(
		queueName, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare a queue")
	confRabbit.Q = q
}

// Publish a message on a queue
func (confRabbit *RabbitMq) PublishMessage(message string) {
	body := message
	err := confRabbit.Ch().Publish(
		"",                // exchange
		confRabbit.Q.Name, // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	log.Printf(" [x] Sent %s", body)
	failOnError(err, "Failed to publish a message")
}

// Consume messages on a queue
func (confRabbit *RabbitMq) ConsumeMessage() {
	msgs, err := confRabbit.Ch().Consume(
		confRabbit.Q.Name, // queue
		"",                // consumer
		true,              // auto-ack
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		log.Printf("Consumer ready, PID: %d", os.Getpid())
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)

			/*addTask := &gopher_and_rabbit.AddTask{}

			err := json.Unmarshal(d.Body, addTask)

			if err != nil {
				log.Printf("Error decoding JSON: %s", err)
			}

			log.Printf("Result of %d + %d is : %d", addTask.Number1, addTask.Number2, addTask.Number1+addTask.Number2)

			if err := d.Ack(false); err != nil {
				log.Printf("Error acknowledging message : %s", err)
			} else {
				log.Printf("Acknowledged message")
			}*/
		}
	}()
	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func setLog() {
	Formatter := new(log.TextFormatter)
	Formatter.TimestampFormat = "02-01-2006 15:04:05"
	Formatter.FullTimestamp = true
	Formatter.ForceColors = true
	log.SetFormatter(Formatter)
	log.SetLevel(log.DebugLevel)
}
