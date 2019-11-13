import copy
import logging
import pprint
import time
import sys

from config.config import Configuration
from config.logging_configuration import LogConfiguration
from drone_agent.orchestration.residual_capacity_utility import ResidualCapacityUtility
from resource_offloading.resource_offload_problem import ResourceOffloadProblem
from drone_agent.orchestration.exceptions import UtilityNotSupported

configuration = None


class DroneOrchestrator:
    """
    This class builds the offload bundle for the given application, bidding for each application component taken.

    """

    def __init__(self, node_name, edge_node_offload_problem):
        """

        :param str node_name: the name of this node
        :param ResourceOffloadProblem edge_node_offload_problem: the instance of the problem
        """

        global configuration
        configuration = Configuration()

        self.enop = edge_node_offload_problem
        self.node_name = node_name

        self.is_deployed = {app: False for app in self.enop.apps}

        self.voting_data = {app: {component: {node: self.init_vote()
                                              for node in self.enop.nodes}
                                  for component in self.enop.app_descriptions[app]['components']}
                            for app in self.enop.apps}
        """ For each app, current votes known of every node on each component """

        self.winners = {app: {component: None
                              for component in self.enop.app_descriptions[app]['components']}
                        for app in self.enop.apps}
        """ Winner nodes computed at the last iteration, for each app and for each component """

        self.per_component_max_vote = {app: {component: sys.maxsize
                                             for component in self.enop.app_descriptions[app]['components']}
                                       for app in self.enop.apps}
        """ Last vote given on each component. Cannot be exceeded during re-voting """

        self.offload_bundle = list()
        """ Contains all the won (app, component, implementation) that the node should allocate """

        self.private_utilities = dict()
        """ Keep track of the partial utilities of each component added to the bundle """

        self.private_utility = None
        """ Object used to compute utility related operations  """

        self.app_blacklist = list()
        """ App whose components are not voted anymore (because they brought to partial allocations). First is worst """
        # TODO apps should be removed from the blacklist in some situations!? (by now it only happens on DEL messages)

        # initialize the node utility implementation according with the specified configuration
        if configuration.PRIVATE_UTILITY == "RESIDUAL-CAPACITY":
            self.private_utility = ResidualCapacityUtility(self.node_name, self.enop)
        else:
            raise UtilityNotSupported("Utility '{}' not supported".format(configuration.PRIVATE_UTILITY))

    def multi_app_election(self, blacklisted_nodes=None):
        """
        
        :param blacklisted_nodes: 
        :return: 
        """
        if blacklisted_nodes is None:
            blacklisted_nodes = set()

        logging.info("****** Start Election ******")
        logging.log(LogConfiguration.VERBOSE, ": blacklisted nodes: " + str(blacklisted_nodes))
        winner_lists = {app: set() for app in self.enop.apps}
        winners = {app: dict() for app in self.enop.apps}
        lost_components = {node: set() for node in self.enop.nodes}
        voted_components = {node: set() for node in self.enop.nodes}

        # compute election for all apps
        apps_offload_bundles = dict()
        for app in self.enop.apps:
            app_winners, app_offload_bundles = self.election(app, blacklisted_nodes)
            winners[app] = app_winners
            app_winner_list = set(app_winners.values())
            logging.debug("app_winner_list: {}".format(app_winner_list))
            winner_lists[app] = app_winner_list
            apps_offload_bundles[app] = app_offload_bundles

        # stores, for each node, lost and voted components
        for node in self.enop.nodes:
            voted_components[node] = self.get_voted_components(node)
            lost_components[node] = {(a, c) for a, c in voted_components[node] if node != winners[a][c]}

        # by now, we don't need any special re-counting, so skip the recursive fix for lost components
        # TODO what if some apps have partial deployment? can we figure out here for sure and re-vote something else?
        # UPDATE: now the worst partial app is removed from the voting and revoting is performed (but only ONCE)

        # Election completed
        logging.info(" WINNERS DICT: '{}'".format(pprint.pformat(winners)))
        logging.info("******* End Election *******")
        return winners, apps_offload_bundles, lost_components

    def election(self, app, blacklisted_nodes=None):
        """
        Election for each component with known votes:
        select winner node with higher vote
        :param str app:
        :param set of str blacklisted_nodes:
        :return: list of winners, node assignment_dict
        """

        if blacklisted_nodes is None:
            blacklisted_nodes = set()

        logging.info("****** Election for app '{}' ******".format(app))
        app_winners = {c: None for c in self.enop.app_descriptions[app]['components']}
        app_offload_bundles = {node: list() for node in self.enop.nodes}
        logging.log(LogConfiguration.VERBOSE,
                    "Voting data: {}".format(pprint.pformat(self.voting_data[app], compact=True)))

        for component in self.enop.app_descriptions[app]['components']:
            logging.debug(" - Component '{}' ...".format(component))
            best_voter = max({k: v for k, v in self.voting_data[app][component].items()
                              if v['value'] > 0 and k not in blacklisted_nodes},
                             key=lambda node: (self.voting_data[app][component][node]['value'],  # highest vote
                                               - self.voting_data[app][component][node]['timestamp']),  # oldest ts
                             default=None)
            # check if we found one
            if best_voter is not None:
                best_vote = self.voting_data[app][component][best_voter]['value']
                implementation = self.voting_data[app][component][best_voter]['implementation']
                # add the voter to the winner dict
                logging.debug(" --- WINNER: '{}' | VOTE: '{}'".format(best_voter, best_vote))
                app_winners[component] = best_voter
                app_offload_bundles[best_voter].append((app, component, implementation))
            else:
                logging.debug(" - No valid vote found for component '{}'.".format(component))

        logging.info(" APP: '{}' | WINNER LIST: '{}'".format(app, pprint.pformat(set(app_winners.values()))))
        logging.info("******* Ended Election for app '{}' *******".format(app))
        return app_winners, app_offload_bundles

    def blacklist_partial_allocations(self, worst_k=1):
        """

        :return: the number of applications that have been blacklisted
        """
        # search for partially allocated apps
        partial_apps = dict()
        for app in self.enop.apps:
            allocated = len([component for component in self.winners[app] if self.winners[app][component] is not None])
            unallocated = len(self.enop.get_app_components(app)) - allocated
            # focus on those for which we reserved resources
            if unallocated > 0 and app in {a for a, c, f in self.offload_bundle}:
                partial_apps[app] = unallocated

        worst_k = min(worst_k, len(partial_apps))
        if len(partial_apps) > 0:
            # sort for number of unallocated components, then for private utility provided
            partial_apps = sorted(partial_apps,
                                  key=lambda x: (partial_apps[x],
                                                 - sum([self.private_utilities[c]
                                                        for c in self.private_utilities
                                                        if c[0] == x])),
                                  reverse=True)
            for i in range(worst_k):
                worst_partial_app = partial_apps[i]
                logging.info("Blacklisting partially allocated application '{}'".format(worst_partial_app))
                # release all components from this app, and blacklist it and try to allocate resources better
                to_release = [(a, c) for a, c, f in self.offload_bundle if a == worst_partial_app]
                self.release_components(to_release)
                self.app_blacklist.append(worst_partial_app)
                logging.info("New offload bundle: {}".format(self.offload_bundle))
        return worst_k

    def orchestrate(self):
        """
        Builds the offload for this node and update the voting data accordingly
        :return:
        """
        '''
        pseudocode:
        while there is room and eligible components:
            fetch the component providing the highest utility
            vote it
            add to the offload bundle
        '''

        logging.info("------------ Starting orchestration process -------------")

        current_offload_bundle = copy.deepcopy(self.offload_bundle)

        # Compute residual resources on this node and fetch any existing vote
        residual_resources = self.enop.available_resources[self.node_name]
        for app, component, implementation in current_offload_bundle:
            if not self.is_deployed[app]:
                residual_resources = self.enop.sub_resources(residual_resources,
                                                             self.enop.get_consumption(implementation))

        # Compute eligible components
        eligible_components = [(app, component)
                               for app in self.enop.apps if not self.is_deployed[app] and app not in self.app_blacklist
                               for component in self.enop.app_descriptions[app]['components']
                               if not self._component_in_bundle(current_offload_bundle, app, component)
                               and self.enop.check_constraints(self.node_name, app, component)
                               and self.per_component_max_vote[app][component] > self._get_winner_vote(app, component)]

        # Build offload bundle
        while True:

            # remove those that does not fit residual resources
            eligible_components = [(app, component) for app, component in eligible_components
                                   if self.enop.is_function_bounded(residual_resources,
                                                                    self.enop.get_implementations(component)[0])]

            # loop break condition
            if len(eligible_components) == 0:
                break

            # 1. Embed next component
            logging.info("Search for next component to add to the bundle ...")
            app, component, implementation, marginal_utility = self._get_next_best_component(current_offload_bundle,
                                                                                             eligible_components)
            logging.info(" --- The next best component has been found: " 
                         "'{}' on app '{}' "
                         "with function '{}' "
                         "giving marginal utility {}"
                         .format(component, app, implementation, marginal_utility))

            # update eligible components
            eligible_components.remove((app, component))

            # 2. Voting selected component
            existing_votes = [self.voting_data[app][component][self.node_name]
                              for component in self.enop.app_descriptions[app]['components']]
            vote_value = self.voting(self.per_component_max_vote[app][component], existing_votes, marginal_utility)
            vote = {'value': vote_value,
                    'implementation': implementation,
                    'timestamp': time.time()}

            # 3. Check if vote is eligible
            best_voter = max({k: v for k, v in self.voting_data[app][component].items() if v['value'] > 0},
                             key=lambda node: (self.voting_data[app][component][node]['value'],  # highest vote
                                               - self.voting_data[app][component][node]['timestamp']),  # oldest ts
                             default=None)
            if best_voter is None:
                best_vote = 0
            else:
                best_vote = self.voting_data[app][component][best_voter]['value']

            if vote_value > best_vote:  # election is performed on the fly here

                logging.debug(" ----- Eligible, added to the bundle.")
                # add to current bundle
                current_offload_bundle.append((app, component, implementation))
                # decrease residual resources accordingly
                residual_resources = self.enop.sub_resources(residual_resources,
                                                             self.enop.get_consumption(implementation))
                # store utility
                self.private_utilities[(app, component, implementation)] = marginal_utility
                # update global voting data
                self.voting_data[app][component][self.node_name] = vote
                self.per_component_max_vote[app][component] = vote_value
                self.winners[app][component] = self.node_name

        # self.winners, apps_offload_bundles, lost_components = self.multi_app_election()
        # self.offload_bundle = self._to_node_bundle(apps_offload_bundles, self.node_name)  # should be redundant!

        self.offload_bundle = current_offload_bundle
        logging.info("Node final offload bundle: {}".format(self.offload_bundle))
        logging.info("------------ End of orchestration process -------------")

    @staticmethod
    def voting(component_old_vote, existing_votes, marginal_utility):
        """
        Generates a vote from a marginal utility so that the overall vote sum is submodular
        :param component_old_vote
        :param existing_votes:
        :param marginal_utility:
        :return:
        """
        # IMPORTANT: value of votes from other apps are here ignored. This is not a problem ONLY if we assume that the
        # utility of a component does not increases due to the presence of components of other applications
        # (we may say this is most likely true)
        marginal_utility = int(round(marginal_utility))
        return min(min([sys.maxsize]+[vote['value'] for vote in existing_votes if vote['value'] > 0]),
                   component_old_vote,
                   marginal_utility)

    @staticmethod
    def _component_in_bundle(offload_bundle, app, component):
        """

        :param offload_bundle:
        :param app:
        :param component:
        :return:
        """
        return len([item for item in offload_bundle if item[0] == app and item[1] == component]) > 0

    @staticmethod
    def _to_node_bundle(apps_offload_bundles, node):
        """

        :param apps_offload_bundles:
        :param node:
        :return:
        """
        node_offload_bundle = list()
        for app in apps_offload_bundles:
            for item in apps_offload_bundles[app][node]:
                node_offload_bundle.append(item)
        return node_offload_bundle

    def _get_next_best_component(self, offload_bundle, eligible_components):
        """

        :param list of tuple offload_bundle:
        :param list of tuple eligible_components:
        :raises NoFunctionsLeft: when is requested to skip mor services/functions than the available
        :return (str, str, str, float): app, component, implementation, marginal utility
        """
        utility_dict = dict()

        for app, component in eligible_components:
            implementation = self.enop.get_implementations(component)[0]
            marginal_utility = self._marginal_utility(offload_bundle, app, component, implementation)
            utility_dict[(app, component, implementation)] = marginal_utility

        # take the one with the highest utility
        best_app, best_component, implementation = sorted(utility_dict, key=lambda x: utility_dict[x], reverse=True)[0]
        marginal_utility = utility_dict[(best_app, best_component, implementation)]
        return best_app, best_component, implementation, marginal_utility

    @staticmethod
    def init_vote(timestamp=0.0):
        """
        Return a 0-vote
        :param timestamp:
        :return dict[str, union[int, str, float]]:
        """
        # TODO replace this structure with a Vote class?
        return {'value': 0, 'implementation': None, 'timestamp': timestamp}

    def get_voted_components(self, node):
        """
        Get components where node has a not-null vote
        :param node:
        :return list of str:
        """
        return [(app, component) for app in self.enop.apps
                for component in self.enop.app_descriptions[app]['components']
                if self.voting_data[app][component][node]['value'] != 0]

    def _marginal_utility(self, offload_bundle, app, component, implementation):
        """
        Compute the marginal utility that node gains by adding given (app,component) to the bundle.
        This function may depend from the particular node.
        :param list of tuple offload_bundle: current bundle of components
        :param str app: application to which the component belong
        :param str component: the component to be added to the current bundle
        :param str implementation: function implementing the component to add to the bundle
        :return: the marginal utility
        """
        # REQUIREMENT: utility of a component must not increase due to the presence of components of other applications
        # (otherwise voting function should be modified by taking into account all votes)
        if not self.enop.check_function_implements_component(component, implementation):
            return 0
        return self.private_utility.marginal_utility(offload_bundle, app, component, implementation)

    def get_node_utility(self):
        """
        Returns the node private utility given the current offload bundle
        :return:
        """
        return sum([self.private_utilities[item] for item in self.offload_bundle])

    def get_winners_list(self, winners_dict=None):
        """
        Returns the winners set merging winners for each node
        :param dict[str, dict[str, str]] winners_dict: if given, use it instead of the current winners
        :return set of str:
        """
        if winners_dict is None:
            winners_dict = self.winners

        return set([winners_dict[app][component] for app in winners_dict for component in winners_dict[app]])

    def _get_winner_vote(self, app, component):
        """

        :param app:
        :param component:
        :return:
        """
        winner = self.winners[app][component]
        if winner is not None:
            return self.voting_data[app][component][winner]['value']
        else:
            return -1

    def sum_votes(self):
        """

        :return:
        """
        return sum([self.voting_data[app][component][node]['value']
                    for app in self.enop.apps
                    for component in self.enop.app_descriptions[app]['components']
                    for node in self.enop.nodes
                    if node == self.winners[app][component]])

    def release_components(self, components, ignore_deployed=True):
        """
        Releases lost components, as well as those whose utility is not valid anymore.
        Also resets the associated votes.
        :param list of tuple components: the tuple is (app, component)
        :param ignore_deployed: do not release those application that already are deployed
        :return:
        """
        # check which components should be released according with the way their utility has been computed
        to_be_released = self.private_utility.to_be_released(self.offload_bundle, components)
        # ignore those that have already been deployed
        if ignore_deployed:
            to_be_released = [(a, c, f) for a, c, f in to_be_released if not self.is_deployed[a]]
        # remove them from the local offload bundle
        self.offload_bundle = [item for item in self.offload_bundle if item not in to_be_released]
        # reset votes and winners for components that have been removed
        for app, component, implementation in to_be_released:
            if self.voting_data[app][component][self.node_name]['value'] != 0:
                self.voting_data[app][component][self.node_name] = self.init_vote(time.time())
            if self.winners[app][component] == self.node_name:
                self.winners[app][component] = None

    def extend_structures_with_app(self, app):
        """

        :param app:
        :return:
        """
        self.is_deployed[app] = False
        self.voting_data[app] = {component: {node: self.init_vote()
                                             for node in self.enop.nodes}
                                 for component in self.enop.app_descriptions[app]['components']}
        self.winners[app] = {component: None for component in self.enop.app_descriptions[app]['components']}
        self.per_component_max_vote[app] = {component: sys.maxsize
                                            for component in self.enop.app_descriptions[app]['components']}

    def remove_app_from_structures(self, app):
        """

        :param app:
        :return:
        """
        self.is_deployed.pop(app)
        self.voting_data.pop(app)
        self.winners.pop(app)
        self.per_component_max_vote.pop(app)

    def get_deployed_bundle(self):
        """

        :return:
        """
        deployed_bundle = [(a, c, f)
                           for a, c, f in self.offload_bundle
                           if self.is_deployed[a]]
        return deployed_bundle

    '''
    def _init_node_votes(self):
        """
        Set to zero all votes for the local node.
        :return:
        """
        for app in self.voting_data:
            for component in self.voting_data[app]:
                self.voting_data[app][component] = self.init_vote(time.time())
    '''