import argparse
import json
import logging
import os

from config.config import Configuration
from config.logging_configuration import LogConfiguration
from resource_offloading.resource_offload_problem import ResourceOffloadProblem
from drone_agent.drone_agent import DroneAgent


def parse_arguments():

    # need to modify global configuration
    global NODE_NAME
    global CONF_FILE
    global LOG_LEVEL
    global LOG_FILE
    global DAEMON_ENABLED

    # define arguments
    parser = argparse.ArgumentParser()
    parser.add_argument(
        'node_name',
        metavar='node-name',
        type=str,
        help='Name of the agent.'
    )
    parser.add_argument(
        '-l',
        '--log-level',
        nargs='?',
        type=str,
        default='INFO',
        help='The logging level.'
    )
    parser.add_argument(
        '-p',
        '--persistent-daemon',
        action='store_true',
        help='If not given, terminates after first agreement is reached.'
    )
    parser.add_argument(
        '-o',
        '--log-on-file',
        action='store_true',
        help='If not given, log will be redirected on stdout.'
    )
    parser.add_argument(
        '-f',
        '--log-file',
        nargs='?',
        default=None,
        help='The log file name. If not given, log will be redirected on stdout.'
    )
    parser.add_argument(
        '-d',
        '--conf-file',
        nargs='?',
        default='config/default-config.ini',
        help='Configuration file.'
    )
    parser.add_argument(
        '-c',
        '--centralized',
        action='store_true',
        help='The log file name. If not given, log will be redirected on stdout.'
    )

    # parse arguments
    args = parser.parse_args()

    NODE_NAME = args.node_name
    CONF_FILE = args.conf_file
    LOG_LEVEL = args.log_level
    LOG_FILE = args.log_file
    DAEMON_ENABLED = args.persistent_daemon
    if LOG_FILE is None and args.log_on_file:
        LOG_FILE = NODE_NAME + ".log"


if __name__ == "__main__":

    parse_arguments()
    configuration = Configuration(CONF_FILE)
    LogConfiguration(LOG_LEVEL, LOG_FILE).configure_log()

    enop = ResourceOffloadProblem()
    with open(configuration.PROBLEM_INSTANCE) as rap_file:
        enop.parse_dict(json.loads(rap_file.read()))
    logging.info(enop)

    # drone agent
    drone = DroneAgent(NODE_NAME, enop, daemon_enabled=DAEMON_ENABLED)

    # Start drone agent
    strong, offloading_bundle, rates = drone.start_agent()

    results = {'offloading-bundle': offloading_bundle,
               'rates': list(rates.items()),
               'utility': int(drone.drone_orchestrator.get_node_utility())}

    results_filename = "{}/results_{}.json".format(configuration.RESULTS_FOLDER, NODE_NAME)
    os.makedirs(os.path.dirname(results_filename), exist_ok=True)
    with open(results_filename, 'w') as f:
        f.write(json.dumps(results, indent=4))

    exit(drone.drone_orchestrator.get_node_utility())
