import argparse
import json
import pika

from config.config import Configuration
from resource_offloading.resource_offload_problem import ResourceOffloadProblem


def purge_queues(_queues, broker, username, password):
    credentials = pika.PlainCredentials(username, password)
    parameters = pika.ConnectionParameters(broker, 5672, '/', credentials)

    connection = pika.BlockingConnection(parameters)
    channel = connection.channel()

    for queue in _queues:
        channel.queue_declare(queue=queue)
        channel.queue_purge(queue)
        channel.queue_delete(queue=queue)
    connection.close()


if __name__ == "__main__":

    parser = argparse.ArgumentParser()
    parser.add_argument(
        '-q',
        '--queues',
        type=str,
        nargs='+',
        help='List of additional queues to purge.',
    )
    parser.add_argument(
        '-d',
        '--conf-file',
        nargs='?',
        default='config/default_config.ini',
        help='Configuration file.'
    )
    args = parser.parse_args()
    additional_queues = list()
    if args.queues:
        additional_queues.extend(args.queues)

    print(args.conf_file)
    print(args.queues)

    configuration = Configuration(args.conf_file)

    with open(configuration.PROBLEM_INSTANCE, mode="r") as enop_file:
        enop = ResourceOffloadProblem()
        enop.parse_dict(json.loads(enop_file.read()))
        nodes = ["{}-drone".format(node) for node in enop.nodes]

    queues = []
    queues.extend(nodes)
    queues.append("{}-drone".format(configuration.APP_ADV_ROUTE))
    queues.append("{}".format(configuration.SOLUTION_ROUTE))
    queues.append("{}".format(configuration.RESOURCE_ROUTE))
    queues.extend(additional_queues)
    print(queues)
    purge_queues(queues, configuration.BROKER_ADDRESS, configuration.USERNAME, configuration.PASSWORD)

