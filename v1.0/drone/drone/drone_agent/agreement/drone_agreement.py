import copy
import itertools
import logging
import pprint

import time

from config.logging_configuration import LogConfiguration
from resource_offloading.resource_offload_problem import ResourceOffloadProblem
from drone_agent.orchestration.drone_orchestrator import DroneOrchestrator


class DroneAgreement:
    """
    This class manage the consensus phase, where each node solves conflict
    based on bidding data received from neighbors.
    """

    def __init__(self, node_name, edge_node_offload_problem, drone_orchestrator):
        """

        :param node_name:
        :param edge_node_offload_problem:
        :param drone_orchestrator:
        :type node_name: str
        :type edge_node_offload_problem: ResourceOffloadProblem
        :type drone_orchestrator: DroneOrchestrator
        :return:
        """
        self.node_name = node_name
        self.enop = edge_node_offload_problem
        self.drone_orchestrator = drone_orchestrator
        self.rebroadcast = False
        self.agreement = False
        self.updated = False
        self.agree_neighbors = set()
        self._pending_revoting = False

    def drone_multi_agreement(self, received_data, revoting_enabled=True):
        """
        Merges data received from multiple senders
        :param dict[str, dict] received_data: dict of received data (the sender is the key)
        :param bool revoting_enabled:
        :return:
        """

        logging.info("--------------- START AGREEMENT ---------------")

        local_voting_data = copy.deepcopy(self.drone_orchestrator.voting_data)
        local_winners = copy.deepcopy(self.drone_orchestrator.winners)

        outvoted = False
        partial_allocation = False
        self.agreement = True
        self.updated = False
        self.rebroadcast = False
        old_per_node_agreement = set(self.agree_neighbors)
        self.agree_neighbors = {node for node in self.agree_neighbors if node not in received_data}

        logging.info(pprint.pformat(self.drone_orchestrator.offload_bundle))

        # add local data in the merging list
        known_data = copy.deepcopy(received_data)
        known_data[self.node_name] = {'voting-data': local_voting_data}

        for app in self.enop.apps:

            logging.info("Conflict resolution for app '" + app + "'")

            # merge all information keeping the most updated one
            merged_data = dict()

            for component in self.enop.app_descriptions[app]['components']:
                merged_data[component] = dict()
                for node in self.enop.nodes:
                    # use the most recent data sent
                    component_data_list = [known_data[sender]['voting-data'][app][component][node]
                                           for sender in known_data.keys()
                                           if app in known_data[sender]['voting-data'].keys()]
                    max_ts = max([component_data['timestamp'] for component_data in component_data_list])
                    merged_data[component][node] = [component_data for component_data in component_data_list
                                                    if component_data['timestamp'] == max_ts][0]

            # compute new winners for this app
            self.drone_orchestrator.voting_data[app] = merged_data

        logging.info("Computing election on new data")
        winners, apps_offload_bundles, lost_components = self.drone_orchestrator.multi_app_election()
        logging.info("Election completed on new data")
        logging.info(pprint.pformat(self.drone_orchestrator.offload_bundle))

        self.drone_orchestrator.winners = winners
        if len(lost_components[self.node_name]) > 0:
            # node has been overbidded
            logging.log(LogConfiguration.IMPORTANT, "Node has been outvoted!")
            logging.info("Lost components {}!".format(lost_components[self.node_name]))
            # release no more valid components
            self.drone_orchestrator.release_components(lost_components[self.node_name])
            # try to repeat bidding on residual resources
            # self.sdo_bidder.sdo_bidding()
            # update & rebroadcast
            outvoted = True

        # if nobody lost anything but there is partial allocation!?
        if len(list(itertools.chain.from_iterable([lost_components[node] for node in lost_components]))) == 0:
            # TODO check if this can be optimized to try use as many resources as possible
            if self.drone_orchestrator.blacklist_partial_allocations():
                partial_allocation = True

        re_voted = False
        # TODO perhaps this check is not needed (is _pending_revoting always True?)
        # if outvoted or partial allocation (need re-voting):
        if revoting_enabled and (self._pending_revoting or outvoted or partial_allocation):
            # try to repeat bidding on residual resources
            self.drone_orchestrator.orchestrate()
            re_voted = True
            self._pending_revoting = False
        elif outvoted or partial_allocation:
            # postpone rebid
            self._pending_revoting = True

        # check nodes agreement
        if re_voted:
            # update & rebroadcast
            logging.log(LogConfiguration.VERBOSE, "UPDATE & REBROADCAST")
            self.rebroadcast = True
            self.agreement = False
            self.updated = True
            logging.info("---------------- END AGREEMENT ----------------")
            return  # if node re-voted, we need to rebroadcast so there is no agreement to check

        send_list = list()

        for sender in received_data.keys():

            logging.info("Checking agreement with node '" + sender + "'")
            self.agree_neighbors.add(sender)

            received_winners = received_data[sender]['winners']
            received_voting_data = received_data[sender]['voting-data']

            # check if the received message changed the situation
            for app in self.enop.apps:

                logging.info(" - Checking data for app '{}'...".format(app))
                if app not in received_voting_data:
                    logging.info("- Still no data for this app from '" + sender + "'!")
                    continue

                logging.info(" - Loc winners: " + str(set(local_winners[app].values())))
                logging.info(" - Rec winners: " + str(set(received_winners[app].values())))
                logging.info(" - New winners: " + str(set(self.drone_orchestrator.winners[app].values())))

                for component in self.enop.app_descriptions[app]['components']:

                    # NOTE: in our decision table "UPDATE" means "keep the merge result"
                    logging.info(" --- Checking component '{}' ...".format(component))
                    agreement_on_component = False

                    local_winner = local_winners[app][component]
                    received_winner = received_winners[app][component]
                    new_winner = self.drone_orchestrator.winners[app][component]

                    local_component_data = local_voting_data[app][component]
                    received_component_data = received_voting_data[app][component]
                    new_component_data = self.drone_orchestrator.voting_data[app][component]

                    if sender == local_winner and self.node_name == received_winner:
                        # i is winner for k and k is winner for i
                        # reset & rebroadcast (*?)
                        logging.log(LogConfiguration.VERBOSE, "RESET & REBROADCAST")
                        self._reset_known_votes(app, component)
                        self.rebroadcast = True
                        self.updated = True
                    elif local_winner == received_winner == new_winner:
                        logging.info(" --- Current winner is equal to received!")
                        if self._compare_bid_times(received_component_data, local_component_data) > 0:
                            # received a new bid time
                            if received_winner is None \
                                    or received_component_data[received_winner] == local_component_data[local_winner]:
                                # some new timestamp but no changes in winner vote
                                agreement_on_component = True
                                self.updated = True
                                logging.log(LogConfiguration.VERBOSE, "UPDATE & NO-REBROADCAST")
                            else:
                                # winner remains the same but there is some change on its vote value
                                # update & rebroadcast
                                # TODO check if this rebroadcast is necessary (it was for resource change in dragon)
                                logging.log(LogConfiguration.VERBOSE, "UPDATE & REBROADCAST")
                                agreement_on_component = True
                                self.rebroadcast = True
                                self.updated = True
                        elif self._compare_bid_times(received_component_data, local_component_data) == 0:
                            # leave & no-rebroadcast
                            agreement_on_component = True
                            logging.log(LogConfiguration.VERBOSE, "LEAVE & NO-REBROADCAST")
                        else:
                            # leave & no-rebroadcast
                            agreement_on_component = True
                            logging.log(LogConfiguration.VERBOSE, "LEAVE & NO-REBROADCAST")
                            # if self.sdo_name not in self.sdo_bidder.winners[node]:
                            #     # leave & no-rebroadcast
                            #     logging.log(LoggingConfiguration.IMPORTANT, "LEAVE & NO-REBROADCAST")
                            # else:
                            #     # leave & rebroadcast
                            #     logging.log(LoggingConfiguration.IMPORTANT, "LEAVE & REBROADCAST")
                            #     self.rebroadcast = True
                    elif received_winner == new_winner:
                        # received winner has overridden local one
                        # update & rebroadcast
                        logging.log(LogConfiguration.VERBOSE, "UPDATE & REBROADCAST")
                        agreement_on_component = True
                        self.rebroadcast = True
                        self.updated = True
                    elif local_winner == new_winner:
                        # winner remains the same
                        logging.info(" --- New winner is same of current but not received")
                        if self.node_name == new_winner:
                            # update-time & rebroadcast
                            logging.log(LogConfiguration.VERBOSE, "UPDATE-TIME & REBROADCAST")
                            # self._update_time(node)   # ?? check if update time is really needed
                            # self.rebroadcast = True     # send only to this node?
                            send_list.append(sender)
                        elif self._compare_bid_times(new_component_data, local_component_data) > 0:
                            # received new ts
                            # update & no-rebroadcast
                            logging.log(LogConfiguration.VERBOSE, "UPDATE & NO-REBROADCAST")
                            self.updated = True
                            # self.rebroadcast = True
                        else:
                            # leave & rebroadcast
                            logging.log(LogConfiguration.VERBOSE, "LEAVE & REBROADCAST")
                            # self.rebroadcast = True     # send only to this node?
                            send_list.append(sender)
                    else:  # new data is different
                        # update & rebroadcast
                        logging.log(LogConfiguration.VERBOSE, "UPDATE & REBROADCAST")
                        self.rebroadcast = True
                        self.updated = True

                    if not agreement_on_component:
                        self.agreement = False
                        self.agree_neighbors.discard(sender)
                    logging.info(" --- Agreement on {}:{} = {}".format(app, component, str(agreement_on_component)))
                # repeat for each component. --)
            # repeat for each app. --)

            logging.info("Agreement with node '" + sender + "':" + str(sender in self.agree_neighbors))

            # if sender not in self.per_node_agreement or sender not in old_per_node_agreement:
            if sender in self.agree_neighbors and sender not in old_per_node_agreement:
                logging.log(LogConfiguration.VERBOSE, "The agreement with this neighbor is new.")
                # self.rebroadcast = True   # send only to this node?
                send_list.append(sender)

        logging.info("---------------- END AGREEMENT ----------------")
        return send_list

    @staticmethod
    def _compare_bid_times(component_voting_data_1, component_voting_data_2, node=None):
        """
        Compares, for each voting node, the timestamps of two voting_data
        :param component_voting_data_1:
        :param component_voting_data_2:
        :param node: if given, compare times just for it
        :type component_voting_data_1: dict[str, dict[str, int]]
        :type component_voting_data_2: dict[str, dict[str, int]]
        :return: 1 if first has at least a newer time, 0 if all are equal, -1 otherwise
        """
        equal_flag = True
        for node_i in component_voting_data_1:
            if node is not None and node_i != node:
                continue
            if component_voting_data_1[node_i]['timestamp'] > component_voting_data_2[node_i]['timestamp']:
                return 1
            elif component_voting_data_1[node_i]['timestamp'] < component_voting_data_2[node_i]['timestamp']:
                equal_flag = False
        if equal_flag:
            return 0
        else:
            return -1

    # should the following three methods stay in the orchestrator module instead?
    def _update_time(self, app=None):
        """

        :param app: if given, updates votes only for the specified app
        """
        for app_i in self.enop.apps:
            if app is None or app == app_i:
                for component in self.enop.app_descriptions[app]['components']:
                    self.drone_orchestrator.voting_data[app][component][self.node_name]['timestamp'] = time.time()

    def _reset_known_votes(self, app, component):
        """

        :param app: the component where the vote should be reset
        :param component: the component where the vote should be reset
        """
        for node in self.drone_orchestrator.voting_data[app][component]:
            self.drone_orchestrator.voting_data[app][component][node] = self.drone_orchestrator.init_vote(time.time())

    def reset_temporary_data(self):
        """

        :return:
        """
        not_deployed_components = [(app, component)
                                   for app in self.enop.apps if not self.drone_orchestrator.is_deployed[app]
                                   for component in self.enop.app_descriptions[app]['components']]
        to_release = [(app, component)
                      for app, component, function in self.drone_orchestrator.offload_bundle
                      if (app, component) in not_deployed_components]
        self.drone_orchestrator.release_components(to_release)
        for app, component in not_deployed_components:
            self._reset_known_votes(app, component)
            self.drone_orchestrator.winners[app][component] = None
        self.rebroadcast = False
        self.agreement = False
        self.updated = False
        self.agree_neighbors = set()
        self._pending_revoting = False
