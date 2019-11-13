import configparser
import os
import inspect
from drone_agent.exceptions import WrongConfigurationFile
from drone_agent.utils.singleton import Singleton


class Configuration(object, metaclass=Singleton):

    def __init__(self, conf_file='config/default_config.ini'):

        self.conf_file = conf_file

        config = configparser.RawConfigParser()
        base_folder = os.path.realpath(os.path.abspath(os.path.split(inspect.getfile(inspect.currentframe()))[0]))\
            .rpartition('/')[0]
        try:
            if base_folder == "":
                config.read(str(base_folder) + self.conf_file)
            else:
                config.read(str(base_folder) + '/' + self.conf_file)

            # [timeout]
            self.AGREEMENT_TIMEOUT = config.getint('timeout', 'agreement_timeout')
            self.WEAK_AGREEMENT_TIMEOUT = config.getint('timeout', 'weak_agreement_timeout')
            self.ASYNC_TIMEOUT = config.getfloat('timeout', 'async_timeout')
            self.SCHEDULING_TIME_LIMIT = config.getint('timeout', 'scheduling_time_limit')
            self.SAMPLE_FREQUENCY = config.getfloat('timeout', 'sample_frequency')

            # [neighborhood]
            self.STABLE_CONNECTIONS = config.getboolean('neighborhood', 'stable_connections')
            self.LOAD_TOPOLOGY = config.getboolean('neighborhood', 'load_topology')
            self.NEIGHBOR_PROBABILITY = config.getint('neighborhood', 'neighbor_probability')
            self.TOPOLOGY_FILE = config.get('neighborhood', 'topology_file')

            # [problem_size]
            self.APPS_NUMBER = config.getint('problem_size', 'apps_number')
            self.NODES_NUMBER = config.getint('problem_size', 'nodes_number')
            self.AVG_APP_SIZE = config.getint('problem_size', 'avg_app_size')

            # [utility]
            self.PRIVATE_UTILITY = config.get('utility', 'private_utility')

            # [logging]
            self.LOG_LEVEL = config.get('logging', 'log_level')
            self.RESULTS_FOLDER = config.get('logging', 'results_folder')

            # [problem]
            self.PROBLEM_INSTANCE = config.get('problem', 'instance')

            # [messaging]
            self.BROKER_ADDRESS = config.get('messaging', 'broker_address')
            self.USERNAME = config.get('messaging', 'username')
            self.PASSWORD = config.get('messaging', 'password')
            self.EXCHANGE_NAME = config.get('messaging', 'exchange_name')
            self.SET_NAME = config.get('messaging', 'set_name')
            self.POLICY_NAME = config.get('messaging', 'policy_name')
            self.SOLUTION_ROUTE = config.get('messaging', 'solution_route')
            self.APP_ADV_ROUTE = config.get('messaging', 'app_advertisement_route')
            self.RESOURCE_ROUTE = config.get('messaging', 'resource_route')
            self.DEBUG_MODE = config.getboolean('messaging', 'debug_mode')

        except Exception as ex:
            raise WrongConfigurationFile(str(ex))
