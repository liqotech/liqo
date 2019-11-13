package messaging

import (
	"drone-operator/drone-operator/pkg/controller/common/configuration"
	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
	"os"
)

var configurationEnv *configuration.ConfigType

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

func InitRabbitMq(conf *configuration.ConfigType) *RabbitMq {

	// Set configuration
	configurationEnv = conf

	// New RabbitMq object
	config := &RabbitMq{}

	config.Connect(configurationEnv.RabbitConf.BrokerAddress, configurationEnv.RabbitConf.BrokerPort, configurationEnv.RabbitConf.VirtualHost, configurationEnv.RabbitConf.Username, configurationEnv.RabbitConf.Password)

	config.CreateChannel()

	config.CreateExchange(configurationEnv.Federation.ExchangeName, amqp.ExchangeDirect)

	config.CreateBindQueue(configurationEnv.RabbitConf.QueueAdvertisementCtrl, configurationEnv.RabbitConf.QueueAdvertisement, configurationEnv.Federation.ExchangeName)

	config.CreateBindQueue(configurationEnv.RabbitConf.QueueAdvertisementDrone, configurationEnv.RabbitConf.QueueAdvertisement, configurationEnv.Federation.ExchangeName)

	config.CreateBindQueue(configurationEnv.RabbitConf.QueueAcknowledgeDeploy+"-"+configurationEnv.Kubernetes.ClusterName, configurationEnv.RabbitConf.QueueAcknowledgeDeploy+"-"+configurationEnv.Kubernetes.ClusterName, configurationEnv.Federation.ExchangeName)

	config.DeclareQueue(configurationEnv.RabbitConf.QueueResult)

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

func (confRabbit *RabbitMq) CreateExchange(exchangeName string, exchangeType string) {
	err := confRabbit.Ch().ExchangeDeclare(exchangeName, exchangeType, false, false, false, false, nil)
	failOnError(err, "Failed to declare exchange")
}

func (confRabbit *RabbitMq) CreateBindQueue(queueName string, routingKey string, exchangeName string) {
	confRabbit.DeclareQueue(queueName)
	err := confRabbit.Ch().QueueBind(queueName, routingKey, exchangeName, false, nil)
	failOnError(err, "Failed to bind queue")
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
func (confRabbit *RabbitMq) PublishMessage(message string, dst string, local bool) {
	body := message
	if local == true {
		// default exchange, for local message
		err := confRabbit.Ch().Publish(
			"",    // exchange
			dst,   // routing key
			false, // mandatory
			false, // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(body),
			})
		log.Printf(" [x] Sent %s", body)
		failOnError(err, "Failed to publish a message")
	} else {
		// default exchange, for local message
		err := confRabbit.Ch().Publish(
			configurationEnv.Federation.ExchangeName, // federate exchange
			dst,                                      // routing key
			false,                                    // mandatory
			false,                                    // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(body),
			})
		log.Printf(" [x] Sent %s", body)
		failOnError(err, "Failed to publish a message")
	}
}

// consumeCallback types take queue name and body string
type consumeCallback func(string, []byte)

// Consume messages on a queue
func (confRabbit *RabbitMq) ConsumeMessage(queueName string, fn func(queueName string, body []byte) error) {
	msgs, err := confRabbit.Ch().Consume(
		queueName, // queue
		"",        // consumer
		true,      // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	failOnError(err, "Failed to register a consumer")

	//forever := make(chan bool)

	go func() {
		log.Printf("Consumer ready, PID: %d", os.Getpid())
		for d := range msgs {
			//log.Printf(" %s: Received a message: %s",queueName, d.Body)
			err := fn(queueName, d.Body)

			/*addTask := &gopher_and_rabbit.AddTask{}

			err := json.Unmarshal(d.Body, addTask)*/

			if err != nil {
				log.Printf("Error decoding JSON: %s", err)
			}

			//log.Printf("Result of %d + %d is : %d", addTask.Number1, addTask.Number2, addTask.Number1+addTask.Number2)

			/*if err := d.Ack(false); err != nil {
				log.Printf("Error acknowledging message : %s", err)
			} else {
				log.Printf("Acknowledged message")
			}*/
		}
	}()
	//log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	//<-forever
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
