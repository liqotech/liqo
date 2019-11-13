import copy
import math
import pprint
import sys


class ResourceOffloadProblem:

    def __init__(self, apps=None, components=None, functions=None, resources=None, nodes=None, consumption=None,
                 total_resources=None, available_resources=None, app_descriptions=None, implementation=None):
        """

        :param apps: app list, index ? of the linear formulation
        :param components: components list, index j of the linear formulation
        :param functions: functions list, index ? of the linear formulation
        :param resources: physical resources list, index k of the linear formulation
        :param nodes: physical nodes list, index i of the linear formulation
        :param consumption: consumption matrix, stores costs in terms of each resource k for function j
        :param total_resources: for eache node n, stores the total amount of each resource k available
        :param available_resources: for eache node n, stores the residual amount of each resource k available
        :param app_descriptions: components for each app
        :param implementation: dict that lists, for each service, all possible functions that can implement it

        :type apps: set of str
        :type components: set of str
        :type functions: set of str
        :type resources: set of str
        :type consumption: dict[str, dict[str, int]]
        :type total_resources: dict[str, dict[str, int]]
        :type available_resources: dict[str, dict[str, int]]
        :type app_descriptions: dict[str, dict[str, union[str, list]]]
        :type implementation: dict[str, (set of str)]
        """

        # indexes
        self.apps = apps
        self.functions = functions
        self.components = components
        self.resources = resources
        self.nodes = nodes

        # problem instance data
        self.consumption = consumption
        self.total_resources = total_resources
        self.available_resources = available_resources
        self.app_descriptions = app_descriptions
        self.implementation = implementation

    def parse_dict(self, rap_dict):

        self.apps = set(rap_dict["apps"])
        self.functions = set(rap_dict["functions"])
        self.components = set(rap_dict["components"])
        self.resources = set(rap_dict["resources"])
        self.nodes = set(rap_dict["nodes"])
        self.consumption = rap_dict["consumption"]
        self.total_resources = rap_dict["available_resources"]
        self.available_resources = copy.deepcopy(self.total_resources)
        self.app_descriptions = rap_dict["app_descriptions"]
        self.implementation = rap_dict["implementations"]

    def to_dict(self):

        rap_dict = dict()
        rap_dict["apps"] = list(self.apps)
        rap_dict["functions"] = list(self.functions)
        rap_dict["components"] = list(self.components)
        rap_dict["resources"] = list(self.resources)
        rap_dict["nodes"] = list(self.nodes)
        rap_dict["consumption"] = self.consumption
        rap_dict["app_descriptions"] = self.app_descriptions
        rap_dict["available_resources"] = self.available_resources
        rap_dict["implementations"] = self.implementation
        return rap_dict

    # old node_assignment structure functions, fix them if needed
    '''
    def check_node_bounded(self, node_assignment_dict, node):
        """
        Checks infrastructure-bounded property of the given assignment for the given node
        :param dict[str, union[int, dict]] node_assignment_dict: the assignment_dict to check,
        is a dict {app:{'bid':int,'consumption':dict}}
        :param str node: the node where the assignment should be bounded
        :return: True if is bounded
        """
        node_assignment_dict_consumption = self.get_node_assignment_dict_consumption(node_assignment_dict)
        for resource in self.resources:
            if node_assignment_dict_consumption[resource] > self.available_resources[node][resource]:
                return False
        return True

    def check_infrastructure_bound(self, assignment_dict):
        """
        Checks infrastructure-bounded property of the given assignment
        :param dict[str, dict[str, union[int, dict]]] assignment_dict: the assignment_dict to check,
        is a dict {node:{sdo:{'bid':int,'consumption':dict}}}
        :return: true if current assignment_dict is infrastructure-bounded
        """
        for node in assignment_dict:
            if not self.check_node_bounded(assignment_dict[node], node):
                return False
        return True

    def check_custom_bound(self, assignment_dict, bounds):
        """
        Checks if the given assignment fit the resource amount given as parameter for each node
        :param dict[str, dict[str, union[int, dict]]] assignment_dict: the assignment_dict to check,
        is a dict {'node':{sdo:{'bid':int,'consumption':dict}}}
        :param dict[str, dict[str, int]] bounds: resources to fit for each node
        :return: true if current assignment_dict is infrastructure-bounded
        """
        for node in set(assignment_dict.keys()):
            if bounds[node] is None:
                return False
            if not self.check_custom_node_bound(assignment_dict[node], bounds[node]):
                return False
        return True

    def check_custom_node_bound(self, node_assignment_dict, bound):
        """

        :param node_assignment_dict:
        :param bound:
        :return:
        """
        node_assignment_dict_consumption = self.get_node_assignment_dict_consumption(node_assignment_dict)
        for resource in self.resources:
            if node_assignment_dict_consumption[resource] > bound[resource]:
                return False
        return True

    def get_residual_resources(self, assignment_dict):
        """
        Get residual resources on node given node_assignment_dict
        :param dict[str, union[int, dict]] assignment_dict: the given assignment_dict,
        is a dict {sdo:{'bid':int,'consumption':dict}}
        :return: the residual resources for each node
        """
        residual_resources = dict()
        for node in self.nodes:
            if node not in assignment_dict:
                residual_resources[node] = self.available_resources[node]
            else:
                residual_resources[node] = self.get_residual_resources_on_node(assignment_dict[node], node)
        return residual_resources

    def get_residual_resources_on_node(self, node_assignment_dict, node):
        """
        Get residual resources on node given node_assignment_dict
        :param dict[str, dict[str, union[int, dict]]] node_assignment_dict: the given assignment_dict,
        is a dict {sdo:{'bid':int,'consumption':dict}}
        :param str node: the node
        :return: the residual resources on the given node
        """
        assignment_dict_consumption = self.get_node_assignment_dict_consumption(node_assignment_dict)
        residual_res = dict()
        for resource in self.resources:
            if assignment_dict_consumption[resource] > self.available_resources[node][resource]:
                return None
            residual_res[resource] = self.available_resources[node][resource] - assignment_dict_consumption[resource]
        return residual_res

    def check_waste_freedom(self):
        """
        Checks waste-free property of the current assignment_dict
        :return: true if current assignment_dict is waste-free
        """
        pass

    def get_node_assignment_dict_consumption(self, node_assignment_dict):
        """

        :param dict[str, union[int, dict]] node_assignment_dict:
        :return dict[str, int]:
        """
        assignment_dict_consumption = {r: 0 for r in self.resources}
        for sdo in node_assignment_dict:
            if 'consumption' in node_assignment_dict[sdo]:
                assignment_dict_consumption = self.sum_resources(assignment_dict_consumption,
                                                                 node_assignment_dict[sdo]['consumption'])
        return assignment_dict_consumption
        
    @staticmethod
    def get_sdo_utility_node_assignment(assignment_dict, sdo):
        """

        :param dict[str, dict[str, union[int, dict]]] assignment_dict: the assignment_dict,
        :param sdo:
        :return:
        """
        overall_utility = 0
        for node in assignment_dict:
            overall_utility += ResourceAssignmentProblem.get_sdo_utility_for_node_assignment(assignment_dict[node], sdo)
        return overall_utility

    @staticmethod
    def get_sdo_utility_for_node_assignment(node_assignment_dict, sdo):
        """

        :param node_assignment_dict:
        :param sdo:
        :return:
        """
        sdo_utility = 0
        for function, utility in node_assignment_dict[sdo]:
            sdo_utility += utility
        return sdo_utility
    '''

    def extend_apps(self, more_apps):
        """

        :param dict[str, dict[str, union[str, list]]] more_apps: apps to be added
        :return:
        """
        for app in more_apps:
            if set(more_apps[app]['components']).issubset(self.components) and app not in self.apps:
                self.apps.add(app)
                self.app_descriptions[app] = more_apps[app]

    def _init_resources(self):
        """

        :return:
        """
        return {r: 0 for r in self.resources}

    def fill_null_resources(self, resource_amount):
        """

        :param dict resource_amount: incomplete resource amount
        :return:
        """
        return {r: resource_amount.get(r, 0) for r in self.resources}

    def get_consumption(self, function):
        """
        Return a dict with all the consumption for the given function
        :param function: the function of which we want to calculate the consumption
        :return:
        """
        return self.consumption[function]

    def sum_resources(self, resources_a, resources_b):
        """

        :param resources_a:
        :param resources_b:
        :return:
        """
        sum_resources = dict()
        for resource in self.resources:
            sum_resources[resource] = resources_a[resource] + resources_b[resource]
        return sum_resources

    def sub_resources(self, resources_a, resources_b):
        """

        :param resources_a:
        :param resources_b:
        :return:
        """
        sub_resources = dict()
        for resource in self.resources:
            sub_resources[resource] = round(resources_a[resource] - resources_b[resource], sys.float_info.dig)
        return sub_resources

    def is_bounded(self, resources_a, resource_b):
        """
        Return true if all resources in a are less/eq then resources in b
        :param resources_a:
        :param resource_b:
        :return:
        """
        for resource in self.resources:
            if resource_b[resource] < resources_a[resource]:
                return False
        return True

    def check_equals(self, resources_a, resources_b):
        """

        :param resources_a:
        :param resources_b:
        :return:
        """
        for resource in self.resources:
            if resources_a[resource] != resources_b[resource]:
                return False
        return True

    def is_function_bounded(self, pool, function):
        """
        Does function fits the pool?
        :param pool:
        :param function:
        :return:
        """
        f_consumption = self.get_consumption(function)
        for resource in self.resources:
            if pool[resource] < f_consumption[resource]:
                return False
        return True

    def get_bundle_consumption(self, bundle_functions):
        """

        :param bundle_functions:
        :return:
        """
        bundle_consumption = self._init_resources()
        for function in bundle_functions:
            bundle_consumption = self.sum_resources(bundle_consumption, self.get_consumption(function))
        return bundle_consumption

    def is_bundle_consuming(self, node, bundle_functions):
        """
        Returns false if the consumption on the node is less than the amount of these functions
        :param node:
        :param bundle_functions:
        :return:
        """
        current_consumption = self.sub_resources(self.total_resources[node], self.available_resources[node])
        bundle_consumption = self._init_resources()
        for function in bundle_functions:
            bundle_consumption = self.sum_resources(bundle_consumption, self.get_consumption(function))
        return self.is_bounded(bundle_consumption, current_consumption)

    def check_function_implements_component(self, service, function):
        """

        :param service:
        :param function:
        :return:
        """
        return function in self.implementation[service]

    def get_implementations(self, component):
        """

        :param component:
        :return:
        """
        return self.implementation[component]

    def get_app_components(self, app):
        """

        :param app:
        :return:
        """
        return self.app_descriptions[app]["components"]

    def norm(self, node, resources):
        """
        Returns a normalized scalar [0, 1] of the given amount of resources with respect with the total node pool
        :param node:
        :param resources:
        :return:
        """
        weighted_quadratic_values = list()
        node_resources = [resource for resource in self.resources if self.total_resources[node][resource] != 0]

        for resource in node_resources:
            # the consumption is scaled so that the contribution of different resources is scaled for their total amount
            consumption = resources[resource]/self.available_resources[node][resource]
            quadratic_value = consumption**2
            weighted_quadratic_values.append(quadratic_value/len(node_resources))   # same weight for each resource

        weighted_quadratic_norm = math.sqrt(sum(weighted_quadratic_values))
        return weighted_quadratic_norm

    def get_overall_available_resources_amount(self):
        """

        :return:
        """
        overall_resources = {resource: 0 for resource in self.resources}

        for node in self.nodes:
            overall_resources = self.sum_resources(overall_resources, self.available_resources[node])
        return overall_resources

    def check_constraints(self, node, app, component):
        """
        Check if the node satisfies all the constraints specified for this component
        :param node:
        :param app:
        :param component:
        :return:
        """
        # check placement constraints
        app_placement_constraints = self.app_descriptions[app]["constraints"]["placement"]
        if component in app_placement_constraints:
            nodes_blacklist = app_placement_constraints[component].get("blacklist")
            nodes_whitelist = app_placement_constraints[component].get("whitelist")
            if nodes_blacklist is not None and node in nodes_blacklist:
                return False
            if nodes_whitelist is not None and node not in nodes_whitelist:
                return False

        return True

    def update_node_resources(self, node, available_resources):
        """
        Update the resources of the given node with the new one. Those that are not listed are kept unchanged.
        :param node:
        :param dict available_resources:
        :return:
        """
        self.resources.update(available_resources.keys())
        for resource in self.resources:
            self.available_resources[node][resource] = available_resources.get(resource, 0)
            if resource not in self.total_resources[node] or self.total_resources[node][resource] == 0 or \
                    self.total_resources[node][resource] < self.available_resources[node][resource]:
                self.total_resources[node][resource] = self.available_resources[node][resource]

    def __str__(self):
        return "\n" \
               "************************* ROP INSTANCE ************************\n" \
               "nodes: " + str(self.nodes) + "\n" \
               "apps: " + str(self.apps) + "\n" \
               "components: " + str(self.components) + "\n" \
               "functions: " + str(self.functions) + "\n" \
               "resources: " + str(self.resources) + "\n" \
               "total resources: \n" + pprint.pformat(self.total_resources) + "\n" \
               "available resources: \n" + pprint.pformat(self.available_resources) + "\n" \
               "consumption: \n" + pprint.pformat(self.consumption, compact=True) + "\n" \
               "implementation map: \n" + pprint.pformat(self.implementation, compact=True) + "\n" \
               "***************************************************************\n" \
               "\n"
