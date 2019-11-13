import logging
import pprint
import time
from collections import OrderedDict

from threading import Lock, Thread, Condition

from config.config import Configuration
from config.logging_configuration import LogConfiguration
from drone_agent.agreement.drone_agreement import DroneAgreement
from drone_agent.messages.app_adv_message import AppAdvMessage
from drone_agent.messages.resource_message import ResourceMessage
from drone_agent.messages.solution_message import SolutionMessage
from drone_agent.orchestration.drone_orchestrator import DroneOrchestrator
from drone_agent.messages.drone_message import DroneMessage
from drone_agent.utils.messaging import Messaging
from drone_agent.utils.neighborhood import NeighborhoodDetector
from resource_offloading.resource_offload_problem import ResourceOffloadProblem

# noinspection PyTypeChecker
configuration = None
""" :type configuration: Configuration """


class DroneAgent:

    def __init__(self, node_name, enop, daemon_enabled=False):
        """

        :param node_name:
        :param ResourceOffloadProblem enop:
        """

        global configuration
        configuration = Configuration()

        # DRONE node
        self.node_name = node_name
        self.enop = enop
        self.drone_orchestrator = DroneOrchestrator(node_name, enop)
        self.drone_agreement = DroneAgreement(node_name, enop, self.drone_orchestrator)
        self.daemon_enabled = daemon_enabled
        self.decision_taken = False

        self.neighborhood_detector = NeighborhoodDetector(nodes=self.enop.nodes,
                                                          base_node=self.node_name,
                                                          load_neighborhood=configuration.LOAD_TOPOLOGY,
                                                          neighbor_probability=configuration.NEIGHBOR_PROBABILITY,
                                                          topology_file=configuration.TOPOLOGY_FILE,
                                                          stable_connections=configuration.STABLE_CONNECTIONS)
        self.neighborhood = self.neighborhood_detector.get_neighborhood()

        # init messaging
        self._messaging = Messaging(configuration.BROKER_ADDRESS)
        self._connection1_id = None

        # message counters
        self.message_counter = 0
        self.received_messages = 0

        # times
        self.begin_time = 0
        self.last_update_time = 0
        self.agreement_time = 0
        self.last_message_time = 0
        self.end_time = 0   # time of the end of the last iteration (both for strong or weak agreement)
        self.last_seen = {node: 0 for node in self.neighborhood}

        # drone messages queues
        self.drone_message_queues = {node: list() for node in self.neighborhood}
        self.queue_locks = {node: Lock() for node in self.neighborhood}  # separated locks per queue, not used
        self.drone_queue_cv = Condition()
        self.pending_rebroadcast = False

        # problem data lock - used to suspend the agreement during structure updating because of an app advertisement
        self.enop_lock = Lock()

        # validation
        self.sent_count = 0  # number of sent messages, only updated after a time window - for validation only purpose
        self.last_time = 0  # time of the last sent message - for validation only purpose
        self.message_rates = OrderedDict()  # messages sent in a each given time window

    def start_agent(self):
        """
        Main distributed scheduling process. There are two threads: one enqueues received messages,
        the other de-queues and handles them.
        :return:
        """

        self.begin_time = time.time()

        # connect
        self._connection1_id = self._messaging.connect(timed=True)

        # fetch initial resources available on node before beginning if they are not given
        if not self.enop.available_resources[self.node_name]:
            logging.info("Waiting for resource updating message...")
            resource_message = self._messaging.consume_blocking(self._connection1_id, configuration.RESOURCE_ROUTE,
                                                                message_class=ResourceMessage, since=self.end_time)
            self.resource_message_handler(resource_message)

        # check if there are applications to be deployed
        to_deploy = [app for app in self.enop.apps
                     if not self.drone_orchestrator.is_deployed[app]
                     if app not in self.drone_orchestrator.app_blacklist]

        if to_deploy:
            # first voting
            self.drone_orchestrator.orchestrate()
            logging.debug(pprint.pformat(self.drone_orchestrator.voting_data))

            self._broadcast(self._connection1_id)
        else:
            self.decision_taken = True

        # here start the daemon that will keep reaching agreement on temporary offloading whenever something changes
        while True:

            if self.end_time > 0:
                self.begin_time = time.time()
                self.message_counter = 0
                self.received_messages = 0
                self.agreement_time = 0
                self.drone_agreement.agree_neighbors = set()

            # the hard timer is set only when a new agreement is required
            if not self.decision_taken:
                # week agreement timeout
                self._messaging.set_stop_timeout(configuration.WEAK_AGREEMENT_TIMEOUT, permanent=True)
                # also set the short timer if no neighbor has spoke recently
                active_neighborhood = {n for n in self.neighborhood if self.last_seen[n] > self.end_time}
                if active_neighborhood.issubset(self.drone_agreement.agree_neighbors):
                    self._messaging.set_stop_timeout(configuration.AGREEMENT_TIMEOUT)
            else:
                # wait for resources to be updated according with new deployment
                bundle_functions = [c[2] for c in self.drone_orchestrator.get_deployed_bundle()]
                if not configuration.DEBUG_MODE:
                    while not self.enop.is_bundle_consuming(self.node_name, bundle_functions):
                        logging.info("Waiting for resources to be updated according with last deployment...")
                        resource_message = self._messaging.consume_blocking(self._connection1_id,
                                                                            configuration.RESOURCE_ROUTE,
                                                                            message_class=ResourceMessage)
                        self.resource_message_handler(resource_message)
                else:
                    bundle_consumption = self.enop.get_bundle_consumption(bundle_functions)
                    self.enop.update_node_resources(self.node_name,
                                                    self.enop.sub_resources(self.enop.total_resources[self.node_name],
                                                                            bundle_consumption))
                logging.info("Resources are updated according with last deployment")

            # register handler for drone messages
            logging.info("Subscribing to receive messages with topic '{}' ...".format(self.node_name))
            self._messaging.register_handler(self._connection1_id, self.node_name, self.enqueue_drone_message,
                                             DroneMessage, refresh_timeout=True)

            # register handler for application-advertisement messages
            logging.info("Subscribing to receive messages with topic '{}' ...".format(configuration.APP_ADV_ROUTE))
            self._messaging.register_handler(self._connection1_id, configuration.APP_ADV_ROUTE,
                                             self.app_advertisement_message_handler, AppAdvMessage)

            # register handler for resource messages
            logging.info("Subscribing to receive messages with topic '{}' ...".format(configuration.RESOURCE_ROUTE))
            self._messaging.register_handler(self._connection1_id, configuration.RESOURCE_ROUTE,
                                             self.resource_message_handler, ResourceMessage, local=True)

            # start to handle messages
            thread = Thread(target=self.drone_messages_consumer_routine)
            thread.start()
            logging.info("Listening for incoming messages ...")
            self._messaging.start_consuming(self._connection1_id)   # returns after the agreement timeout expires

            active_neighborhood = {n for n in self.neighborhood if self.last_seen[n] > self.end_time}
            strong_agreement = active_neighborhood.issubset(self.drone_agreement.agree_neighbors)
            logging.info("Stop consuming messages.")

            # agreement completed
            logging.log(LogConfiguration.IMPORTANT, "Waiting for listener thread to terminate ...")
            self.drone_queue_cv.acquire()
            self.end_time = time.time()
            self.drone_queue_cv.notify()
            self.drone_queue_cv.release()
            thread.join()

            logging.log(LogConfiguration.IMPORTANT,
                        "Agreement process has been completed! (strong={})".format(str(strong_agreement)))

            logging.info("New offload bundle: {}".format(self.drone_orchestrator.offload_bundle))

            # if there have been no response, fix any partial allocation
            if not active_neighborhood:
                logging.log(LogConfiguration.IMPORTANT, "No response from neighborhood!")
                while self.drone_orchestrator.blacklist_partial_allocations():
                    self.drone_orchestrator.orchestrate()

            self.decision_taken = True

            # -------------- send a message with the scheduled offload information --------------
            if not configuration.DEBUG_MODE:
                solution = self._build_solution_message_from_offload_bundle(self.drone_orchestrator.offload_bundle,
                                                                            self.drone_orchestrator.winners)
                self._messaging.send_message(self._connection1_id, configuration.SOLUTION_ROUTE, solution, local=True)
                logging.info("Offloading solution information has been sent.")
            # --------------------------------------------------------------------------------------

            if self.node_name in self.drone_orchestrator.get_winners_list():
                logging.info(" - Node '{}' got to implement some component functions!".format(self.node_name))
                logging.info(" - Assigned components are: \n" +
                             pprint.pformat(self.drone_orchestrator.offload_bundle))
            else:
                logging.info(" - Node '{}' does not have to implement any component function.".format(self.node_name))
            instantiated_apps = [app for app in self.drone_orchestrator.winners
                                 if None not in self.drone_orchestrator.winners[app].values()]

            # mark apps as deployed in orchestrator object
            for app in instantiated_apps:
                self.drone_orchestrator.is_deployed[app] = True

            print(self.node_name.ljust(6) +
                  " | strong: " + str(strong_agreement).ljust(5) +
                  " | instantiated apps: " + str(sorted(set(instantiated_apps))) +
                  " | V: " + str(int(self.drone_orchestrator.sum_votes())) +
                  " | u: " + str(self.drone_orchestrator.get_node_utility()).rjust(3) +
                  " | last update on: " + str(self.last_update_time - self.begin_time)[:5] +
                  " | agreement on: " + str(self.agreement_time - self.begin_time)[:5] +
                  " | last message on: " + str(self.last_message_time - self.begin_time)[:5] +
                  " | total time: " + str(self.end_time - self.begin_time)[:5] +
                  " | sent messages: " + str(self.message_counter).rjust(7) +
                  " | received messages: " + str(self.received_messages).rjust(7))

            if not self.daemon_enabled:
                break

        # disconnect
        self._messaging.disconnect(self._connection1_id)
        return strong_agreement, self.drone_orchestrator.offload_bundle, self.message_rates

    def drone_messages_consumer_routine(self):
        """
        Main thread routine, waits for messages to arrive in the queue and invokes the handler
        :return:
        """

        last_end_time = self.end_time
        while self.end_time == last_end_time:
            messages = self._dequeue_next_messages(last_end_time=last_end_time)
            if not messages:
                continue
            self._drone_messages_handler(messages)

    def enqueue_drone_message(self, message):
        """
        This is the callback invoked by the messaging module, to make a message available to the message_listener
        :param DroneMessage message:
        :return:
        """
        self.last_message_time = time.time()
        self.drone_queue_cv.acquire()
        # self.queue_locks[message.sender].acquire()
        self.drone_message_queues[message.sender].append(message)
        # self.queue_locks[message.sender].release()
        self.drone_queue_cv.notify_all()
        self.drone_queue_cv.release()

    def _dequeue_next_messages(self, last_end_time):
        """

        :return:
        """
        self.drone_queue_cv.acquire()

        timeout = float(configuration.ASYNC_TIMEOUT)
        while timeout > 0 \
                and len([q for q in self.drone_message_queues
                         if q not in self.drone_agreement.agree_neighbors and len(self.drone_message_queues[q]) == 0]
                        ) > 0 \
                and self.end_time == last_end_time:
            start_t = time.time()
            self.drone_queue_cv.wait(timeout)
            end_t = time.time()
            timeout -= end_t-start_t

        if self.end_time != last_end_time:
            self.drone_queue_cv.release()
            return list()

        messages = list()
        for node in self.neighborhood:
            # self.queue_locks[node].acquire()
            if len(self.drone_message_queues[node]) > 0:
                # consider just the last message for each neighbor
                message = self.drone_message_queues[node][-1]
                # ignore old messages
                if message.timestamp > self.begin_time:
                    messages.append(message)
                self.drone_message_queues[node] = list()
            # self.queue_locks[node].release()
        self.drone_queue_cv.release()
        return messages

    def _drone_messages_handler(self, messages):
        """
        Calls an agreement function that merges, at once, data received from multiple messages
        :param messages:
        :type messages: list of DroneMessage
        :return:
        """

        # update the overall number of messages
        self.received_messages += len(messages)
        # fetch the current active neighborhood
        active_neighborhood = {n for n in self.neighborhood if self.last_seen[n] > self.end_time}
        senders = {m.sender for m in messages}

        with self.enop_lock:    # what if the message is related to an older version of the problem?

            # update last-seen timestamps according with new messages received from peers
            for message in messages:
                self.last_seen[message.sender] = message.timestamp
                # blacklisted applications may have a chance if this is a new node
                if message.sender not in active_neighborhood and self.drone_orchestrator.app_blacklist:
                    self.drone_orchestrator.app_blacklist = list()
                    # repeat voting on previously blacklisted apps
                    self.drone_orchestrator.orchestrate()
                    logging.debug(pprint.pformat(self.drone_orchestrator.voting_data))

            # [ perform agreement process for these messages ]

            self.decision_taken = False
            previous_agreement = active_neighborhood.issubset(self.drone_agreement.agree_neighbors)
            logging.log(LogConfiguration.IMPORTANT, "Handling messages from {}".format(",".join(senders)))
            data = {m.sender: {'voting-data': m.voting_data, 'winners': m.winners} for m in messages}
            send_list = self.drone_agreement.drone_multi_agreement(data)

            # [ rebroadcast ]
            connection_id = self._messaging.connect()
            if self.drone_agreement.rebroadcast:
                self._broadcast(connection_id)
            elif send_list:
                self._broadcast(connection_id, send_list)
            else:
                logging.info("No need to rebroadcast voting information.")
            self._messaging.disconnect(connection_id)

            # [ agreement check ]
            if self.drone_agreement.updated:
                self.last_update_time = time.time()
            # check if an agreement present at last iteration has now been broken
            if previous_agreement:
                if not self.drone_agreement.agreement or not senders.issubset(active_neighborhood):
                    # old agreement has been broken
                    logging.log(LogConfiguration.IMPORTANT, "Previous agreement has been broken.")
                    self.agreement_time = 0
                    # delete timeout if any
                    self._messaging.del_stop_timeout()
            for message in messages:
                if message.sender in self.drone_agreement.agree_neighbors:
                    # NEIGHBOR AGREEMENT - data that neighbor sent is consistent with local
                    logging.log(LogConfiguration.IMPORTANT,
                                "Agreement reached with neighbor '{}'".format(message.sender))

            active_neighborhood = {n for n in self.neighborhood if self.last_seen[n] > self.end_time}
            if active_neighborhood.issubset(self.drone_agreement.agree_neighbors):
                # NEIGHBORHOOD AGREEMENT - data that all neighbor sent are consistent with local
                if self.agreement_time == 0:
                    # this agreement is new
                    logging.log(LogConfiguration.IMPORTANT, "=====================================")
                    logging.log(LogConfiguration.IMPORTANT, "AGREEMENT REACHED WITH NEIGHBORHOOD!!")
                    logging.log(LogConfiguration.IMPORTANT, "=====================================")
                    self.agreement_time = time.time()
                    # set timeout to stop wait messages if nothing new arrives
                    logging.log(LogConfiguration.IMPORTANT, " - Waiting {} seconds for new messages before ending ..."
                                .format(configuration.AGREEMENT_TIMEOUT))
                    self._messaging.set_stop_timeout(configuration.AGREEMENT_TIMEOUT)
                else:
                    logging.info("Confirmed existing agreement with neighborhood.")

    def _broadcast(self, connection_id, neighborhood=None):
        """
        Send local voting data to each neighbor.
        """
        logging.info("Broadcasting voting information ...")

        # build the message to broadcast
        message_to_broadcast = DroneMessage(sender=self.node_name,
                                            winners=self.drone_orchestrator.winners,
                                            voting_data=self.drone_orchestrator.voting_data)

        # get the neighbors list
        if neighborhood is None:
            neighborhood = self.neighborhood_detector.get_current_neighborhood()

        # time.sleep(0.06)

        for neighbor in neighborhood:
            logging.info("Sending message to neighbor '" + neighbor + "' ...")
            self._messaging.send_message(connection_id, neighbor, message_to_broadcast)
            self.message_counter += 1
            logging.log(LogConfiguration.VERBOSE, "Message has been sent.")

        # store rate for validation
        timestamp = time.time()
        sent_time = float("{0:.3f}".format(timestamp))
        if len(self.message_rates) == 0:
            self.last_time = float("{0:.3f}".format(self.begin_time))
        else:
            last_begin_time = float(next(reversed(self.message_rates)).split(":")[0])
            if last_begin_time == self.last_time:
                del self.message_rates[next(reversed(self.message_rates))]

        if sent_time - self.last_time > configuration.SAMPLE_FREQUENCY:
            self.message_rates[str(self.last_time) + ":" + str(sent_time)] = self.message_counter - self.sent_count
            self.sent_count = self.message_counter
            self.last_time = sent_time
        else:
            self.message_rates[str(self.last_time) + ":" + str(sent_time)] = self.message_counter - self.sent_count

        logging.info("Broadcast successfully completed.")

    '''
        APPLICATION ADVERTISEMENT
    '''

    def app_advertisement_message_handler(self, message):
        """
        This is the callback invoked by the messaging module, any time an app advertisement message arrives.
        It updates the drone structures accordingly and sets the requirement for a new agreement
        :param AppAdvMessage message:
        :return:
        """

        if not self._validate_app_adv_message_against_problem(message, self.enop):
            # ignore advertisement since it is not coherent with local problem
            logging.warning("Ignoring invalid application advertisement ...")
            return

        logging.log(LogConfiguration.IMPORTANT, "New application advertisement, updating problem data ...")

        with self.enop_lock:

            # reset agreement and orchestration data
            self.drone_agreement.reset_temporary_data()

            active_neighborhood = {n for n in self.neighborhood if self.last_seen[n] > self.end_time}

            # invalidate any current agreement
            if active_neighborhood.issubset(self.drone_agreement.agree_neighbors):
                # old agreement is no more valid
                logging.info("Previous agreement is no more valid.".format(message.app_name))
                self.agreement_time = 0
                # delete timeout if any
                self._messaging.del_stop_timeout()
            self.drone_agreement.agree_neighbors = set()
            self.decision_taken = False

            if message.adv_type == AppAdvMessage.AdvertisementType.DEL:

                logging.info("Advertisement: application '{}' deleted.".format(message.app_name))

                # blacklisted applications may have a chance because of the new room
                self.drone_orchestrator.app_blacklist = list()

                if self.drone_orchestrator.is_deployed[message.app_name]:
                    # release any component of this app that is in the local bundle
                    to_release = [(app, component)
                                  for app, component, function in self.drone_orchestrator.offload_bundle
                                  if app == message.app_name]
                    self.drone_orchestrator.is_deployed[message.app_name] = False
                    self.drone_orchestrator.release_components(to_release)

                # update the problem structure
                self.enop.apps.remove(message.app_name)
                self.enop.app_descriptions.pop(message.app_name)

                # update orchestration structures
                self.drone_orchestrator.remove_app_from_structures(message.app_name)

            elif message.adv_type == AppAdvMessage.AdvertisementType.ADD:

                logging.info("Advertisement: new application '{}'.".format(message.app_name))

                # update the problem structure
                self._extend_enop_apps_from_message(message)

                # update orchestration structures
                self.drone_orchestrator.extend_structures_with_app(message.app_name)

            elif message.adv_type == AppAdvMessage.AdvertisementType.MOD:

                logging.info("Advertisement: application '{}' has been modified.".format(message.app_name))
                logging.warning("Update of existing apps is not implemented, the message is ignored.")
                # this is not supported by now, ignoring ...
                return

            # repeat voting on new structures
            self.drone_orchestrator.orchestrate()
            logging.debug(pprint.pformat(self.drone_orchestrator.voting_data))
            self._broadcast(self._connection1_id)

            # set (or update) the hard timer, since a new agreement is required
            self._messaging.set_stop_timeout(configuration.WEAK_AGREEMENT_TIMEOUT, permanent=True)
            # also set the shortest timer if no neighbor has spoke recently
            if active_neighborhood.issubset(self.drone_agreement.agree_neighbors):
                self._messaging.set_stop_timeout(configuration.AGREEMENT_TIMEOUT)

    def _extend_enop_apps_from_message(self, message):
        """

        :param message:
        :return:
        """

        app_description = dict()
        app_description["base-node"] = message.base_node
        app_description["components"] = list()
        app_description["constraints"] = {"placement": dict()}
        for component in message.components:
            app_description["components"].append(component.name)
            if component.name not in self.enop.components:
                # add the new component to the known ones
                self.enop.components.add(component.name)
                self.enop.implementation[component.name] = list()
            if component.function is not None:
                # add the new function to the known ones
                function_name = "{}:{}".format(component.function.image, component.name)
                self.enop.functions.add(function_name)
                extended_consumption = self.enop.fill_null_resources(component.function.consumption)
                self.enop.consumption[function_name] = extended_consumption
                self.enop.implementation[component.name].append(function_name)
            app_description["constraints"]["placement"][component.name] = {"blacklist": None, "whitelist": None}
            if component.nodes_blacklist is not None:
                # noinspection PyTypeChecker
                app_description["constraints"]["placement"][component.name]["blacklist"] = component.nodes_blacklist
            if component.nodes_whitelist is not None:
                # noinspection PyTypeChecker
                app_description["constraints"]["placement"][component.name]["whitelist"] = component.nodes_whitelist

        self.enop.apps.add(message.app_name)
        self.enop.app_descriptions[message.app_name] = app_description

    @staticmethod
    def _validate_app_adv_message_against_problem(message, enop):
        """

        :param AppAdvMessage message:
        :param ResourceOffloadProblem enop:
        :return:
        """
        if message.adv_type == AppAdvMessage.AdvertisementType.DEL:

            if message.app_name not in enop.apps:
                return False

        elif message.adv_type == AppAdvMessage.AdvertisementType.ADD:

            if message.app_name in enop.apps:
                return False
            for component in message.components:
                if component.name not in enop.components and component.function is None:
                    return False
                if component.function is not None:
                    if not set(component.function.consumption.keys()).issubset(enop.resources):
                        return False

        elif message.adv_type == AppAdvMessage.AdvertisementType.MOD:
            return True

        return True

    '''
        RESOURCE UPDATE
    '''

    def resource_message_handler(self, message):
        """
        This is the callback invoked by the messaging module, any time a resource update message arrives.
        It updates the problem structures accordingly.
        :param ResourceMessage message:
        :return:
        """
        logging.log(LogConfiguration.IMPORTANT, "New node resource message, updating problem data ...")

        with self.enop_lock:

            self.enop.update_node_resources(self.node_name, message.node_resources)

            # What else to do? For now it is assumed resources can changes only as a drone result,
            # thus this update causes no problem on the algorithm execution.

        logging.debug(self.enop)

    '''
        SOLUTION ADVERTISEMENT
    '''

    def _build_solution_message_from_offload_bundle(self, offload_bundle, overall_offloading):
        """

        :param offload_bundle:
        :param overall_offloading:
        :return:
        """
        local_offloading = [SolutionMessage.OffloadedComponent(name=c,
                                                               application=a,
                                                               function=SolutionMessage.Function(
                                                                   f, self.enop.consumption[f]))
                            for a, c, f in offload_bundle]
        return SolutionMessage(self.node_name, local_offloading, overall_offloading)
