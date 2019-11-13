import json
import logging
import uuid
from json import JSONDecodeError

import pika
from pika.adapters.blocking_connection import BlockingChannel
from pika.adapters.blocking_connection import BlockingConnection

from config.logging_configuration import LogConfiguration
from drone_agent.messages.exceptions import InvalidMessageError
from drone_agent.messages.message import Message
from drone_agent.utils.exceptions import MultipleTimedConnectionNotSupportedError, ConnectionNotFoundError
from drone_agent.utils.singleton import Singleton

from config.config import Configuration


class Messaging(object, metaclass=Singleton):
    """
    This class manages exchange of messages with the neighborhood through rabbitmq.
    :type _channels: dict[str, BlockingChannel]
    :type _connections: dict[str, BlockingConnection]
    """

    def __init__(self, broker=None):
        """

        :param str broker: address of the broker
        """
        conf = Configuration()
        self._broker_host = broker
        self._timeout_id = None
        self._permanent_timeout_id = None
        self._permanent_timeout = None  # the timeout in seconds
        self._connections = dict()
        self._channels = dict()
        self._timed_connection_id = None
        self._exchange_name = conf.EXCHANGE_NAME
        self._message_handlers = dict()
        self._message_handlers['default'] = self._default_message_handler
        self._message_classes = dict()
        self._timeout_refresher_topics = list()

    def connect(self, timed=False, local=False):
        """
        Creates a new connection and returns an identifier.
        :param timed:
        :param local:
        :return:
        """
        conf = Configuration()
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        # create an unique identifier for this connection
        connection_id = uuid.uuid4().hex
        if timed:
            if self._timed_connection_id is not None:
                raise MultipleTimedConnectionNotSupportedError("A timed connection already exists.")
            else:
                self._timed_connection_id = connection_id
        connection = self._rabbitmq_connect(self._broker_host, conf.USERNAME, conf.PASSWORD)
        channel = connection.channel()
        # create exchange for federation
        if not local:
            channel.exchange_declare(self._exchange_name, 'direct')

        self._connections[connection_id] = connection
        self._channels[connection_id] = channel
        return connection_id

    def disconnect(self, connection_id):
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        connection = self._connections.pop(connection_id)
        connection.close()
        self._channels.pop(connection_id)
        if connection_id == self._timed_connection_id:
            self._timed_connection_id = None

    @staticmethod
    def _rabbitmq_connect(broker_host, username, password):
        """

        :param broker_host:
        :return:
        """
        credentials = pika.PlainCredentials(username, password)
        parameters = pika.ConnectionParameters(broker_host, 5672, '/', credentials)
        return pika.BlockingConnection(parameters)

    def set_stop_timeout(self, timeout, permanent=False):
        """
        Set a timeout to stop consuming messages. If the timeout is permanent and is already set, then it is refreshed.
        If the timeout is not permanent and is already set, nothing is performed.
        :param int timeout:
        :param bool permanent:
        :return:
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        connection_id = self._timed_connection_id
        if permanent:
            if self._permanent_timeout_id is not None:
                self._permanent_timeout_id = self._refresh_timeout(self._permanent_timeout_id, timeout)
            else:
                self._permanent_timeout_id = self._connections[connection_id].add_timeout(timeout,
                                                                                          self._timeout_handler)
                self._permanent_timeout = timeout
        elif self._timeout_id is None:
            self._timeout_id = self._connections[connection_id].add_timeout(timeout, self._timeout_handler)

    def del_stop_timeout(self):
        """
        Delete the current timeout
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        connection_id = self._timed_connection_id
        if self._timeout_id is not None:
            self._connections[connection_id].remove_timeout(self._timeout_id)
            self._timeout_id = None

    def _refresh_timeout(self, timeout_id, timeout):
        """
        Reset the permanent timeout
        :param timeout_id:
        :param timeout:
        :return: the new id
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        connection_id = self._timed_connection_id
        if timeout_id is not None:
            self._connections[connection_id].remove_timeout(timeout_id)
        return self._connections[connection_id].add_timeout(timeout, self._timeout_handler)

    def _timeout_handler(self):
        """
        Stops message consuming
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        logging.log(LogConfiguration.IMPORTANT, "Timeout!")
        # remove any other timeout
        if self._timeout_id is not None:
            self._connections[self._timed_connection_id].remove_timeout(self._timeout_id)
            self._timeout_id = None
        if self._permanent_timeout_id is not None:
            self._connections[self._timed_connection_id].remove_timeout(self._permanent_timeout_id)
            self._permanent_timeout_id = None
        self.stop_consuming(self._timed_connection_id)

    # not used
    def _timeout_handler_p(self):
        """
        Stops message consuming
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        logging.log(LogConfiguration.IMPORTANT, "Timeout P!")
        self.stop_consuming(self._timed_connection_id)

    def send_message(self, connection_id, dst, message, local=False):
        """
        :param str connection_id:
        :param str dst: name of the destination is used as queue and topic
        :param Message message:
        :param bool local: if true, a non federated exchange is used (for now it is '')
        :return:
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        if connection_id not in self._channels:
            raise ConnectionNotFoundError(connection_id)
        if local:
            exchange = ''
            queue = dst
        else:
            exchange = self._exchange_name
            queue = "{}-drone".format(dst)
        message_body = json.dumps(message.to_dict())
        self._channels[connection_id].queue_declare(queue=queue)
        if exchange != '':
            self._channels[connection_id].queue_bind(exchange=exchange, queue=queue, routing_key=dst)
        self._channels[connection_id].basic_publish(exchange=exchange, routing_key=dst, body=message_body)

    def start_consuming(self, connection_id):
        """
        :return:
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        if connection_id not in self._channels:
            raise ConnectionNotFoundError(connection_id)
        self._channels[connection_id].start_consuming()

    def stop_consuming(self, connection_id):
        """

        :return:
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        if connection_id not in self._channels:
            raise ConnectionNotFoundError(connection_id)
        self._channels[connection_id].stop_consuming()

    def consume_blocking(self, connection_id, topic, local=True, message_class=None, since=0):
        """
        Consume a single message from the queue.
        Blocks on empty queue.
        :param connection_id:
        :param topic:
        :param local: if true, the local exchange is used, else the federated one
        :param message_class: the class used to parse the message
        :param since: discard all messages older than this timestamp
        :return:
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())

        if local:
            queue = topic
        else:
            queue = "{}-drone".format(topic)
        if message_class is None:
            message_class = Message

        message = None
        self._channels[connection_id].queue_declare(queue=queue)
        for method, properties, body in self._channels[connection_id].consume(queue=queue):
            self._channels[connection_id].basic_ack(method.delivery_tag)
            message = message_class()
            message.parse_dict(json.loads(body.decode()))
            if message.timestamp > since:
                break
        self._channels[connection_id].cancel()
        return message

    # not used
    def consume_not_blocking(self, connection_id, topic, local=True):
        """
        Consume a single message from the queue. Does not block on empty queue.
        :return:
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())

        if local:
            queue = topic
        else:
            queue = "{}-drone".format(topic)

        self._channels[connection_id].queue_declare(queue=queue)
        method, properties, body = self._channels[connection_id].basic_get(queue=queue)
        if method is None:
            return None
        self._channels[connection_id].basic_ack(method.delivery_tag)

        message = Message()
        message.parse_dict(json.loads(body.decode()))

        return message

    def register_handler(self, connection_id, topic, handler=None, message_class=None, refresh_timeout=False,
                         local=False):
        """

        :param connection_id:
        :param topic:
        :param handler: must be a a callable that takes one parameter of type BiddingMessage
        :param message_class: the class to be used to parse the message (should extend the Message class)
        :param refresh_timeout: if true causes the permanent timeout to be refreshed at every received message
        :param bool local: if true, a non federated exchange is used (for now it is '')  # never used
        :return:
        """
        # logging.log(LoggingConfiguration.IMPORTANT, threading.get_ident())
        if connection_id not in self._channels:
            raise ConnectionNotFoundError(connection_id)

        if local:
            exchange = ''
            queue = topic
        else:
            exchange = self._exchange_name
            queue = "{}-drone".format(topic)

        if refresh_timeout:
            self._timeout_refresher_topics.append(topic)

        if handler is None:
            handler = self._default_message_handler

        self._channels[connection_id].queue_declare(queue=queue)
        if exchange != '':
            self._channels[connection_id].queue_bind(exchange=exchange, queue=queue, routing_key=topic)
        self._message_handlers[topic] = handler
        self._message_classes[topic] = message_class
        self._channels[connection_id].basic_consume(self._message_callback, queue=queue, no_ack=True)

    @staticmethod
    def _message_callback(channel, method, header, body):
        """

        :param pika.channel.Channel channel: The channel object
        :param pika.Spec.Basic.Deliver method: basic_deliver method
        :param pika.Spec.BasicProperties header: properties
        :param bytes body: The message body
        :return:
        """
        topic = method.routing_key

        message_dict = body.decode()
        logging.log(15, " [x] Received " + message_dict)
        self = Messaging()  # this class is singleton

        # refresh timeout if needed
        if topic in self._timeout_refresher_topics:
            self._permanent_timeout_id = self._refresh_timeout(self._permanent_timeout_id, self._permanent_timeout)

        # fetch the correct handler and message class
        message_class = self._message_classes[topic]
        message_handler = self._message_handlers[topic]

        try:
            message = message_class()
            message.parse_dict(json.loads(message_dict))
            message_handler(message)
        except InvalidMessageError as err:
            logging.warning("Ignoring received invalid message: {}".format(err))
        except JSONDecodeError as err:
            logging.warning("Received invalid non-json message: {}".format(err))

    @staticmethod
    def _default_message_handler(message):
        """

        :param Message message:
        """
        print("Received message from '" + message.sender + "': \n" + str(message.to_dict()))
