from abc import ABCMeta, abstractmethod


class PrivateUtilityInterface:
    """
    Abstract definitions of the methods that should be provided by a custom utility implementation.
    """
    __metaclass__ = ABCMeta

    def __init__(self, node, edge_node_offload_problem):
        """

        :param node:
        :param edge_node_offload_problem:
        """
        self.node_name = node
        self.enop = edge_node_offload_problem

    @abstractmethod
    def marginal_utility(self, offloading_bundle, app, component, implementation):
        """
        Returns the marginal utility of adding the given component to the current offloading bundle.
        :param offloading_bundle:
        :param app:
        :param component:
        :param implementation:
        :return:
        :rtype: float
        """
        raise NotImplementedError

    @abstractmethod
    def to_be_released(self, offloading_bundle, lost_components):
        """
        Given the list of components lost during the election, returns the complete list of components that should be
        removed from the bundle. It should include components whose marginal utility was dependent from the lost ones.
        :param offloading_bundle:
        :param lost_components:
        :return: the list of components whose utility is not valid anymore
        :rtype: list of tuple
        """
        raise NotImplementedError

    # ------------------------------ #

    # following functions may be useful for certain complex utility implementations (they where used in dragon)
    '''
    def _get_transformation(self, components, functions):
        """
        Returns, pseudo-random, a transformation based on inputs
        :param components:
        :param functions:
        :return:
        """
        components = [""] + components
        functions = [""] + functions
        pr_1 = int(hashlib.sha256(reduce(lambda x, y: x + y, components + functions).encode('utf-8')).hexdigest(), 16)
        pr_2 = int(hashlib.sha256(reduce(lambda x, y: x + y, functions + components).encode('utf-8')).hexdigest(), 16)
        normalized_pr1 = pr_1/2**256
        normalized_pr2 = pr_2/2**256
        # bits_pr = bin(pr_2)[2:]

        # choose transformation
        if normalized_pr1 > 0.5:
            # polynomial transformation
            power = (5 - 1) * normalized_pr2 + 1
            root = int(normalized_pr1*100) % 2 == 0
            return self._polynomial_transformation, [power, root]
        else:
            # sin transformation
            sin_sign = int(normalized_pr1*100) % 2 == 0
            b = int((5 - 2) * normalized_pr2 + 2)
            a = int((5 - b) * normalized_pr1 + b)
            if not sin_sign:
                a = -a
            return self._x_sin_transformation, [a, b]

    @staticmethod
    def _polynomial_transformation(x, a, root=False):
        """
        Scales input value according to a pow transformation
        :param x: bounded in [0, 1]
        :param a: the pow exponent
        :param root: if true, transform according to a root curve instead of a polynomial one
        :return: the transformed value
        """
        if root:
            a = 1/a
        return x**a

    @staticmethod
    def _x_sin_transformation(x, a, b):
        """
        Scales input value according to a x*sin transformation
        :param x: bounded in [0, 1]
        :param a: modulates sin amplitude (higher is smaller)
        :param a: modulates sin frequency (higher is faster)
        :return: the transformed value
        """
        return x + 1/(a*math.pi)*math.sin(b*math.pi*x)

    @staticmethod
    def _gen_log_func(x, a, k, b, v, q, c=1):
        """

        :param x: the function parameter
        :param a: the lower asymptote
        :param k: the upper asymptote.
        :param b: the growth rate
        :param v: affects near which asymptote maximum growth occurs.
        :param q: is related to the value Y(0)
        :param c:
        :return:
        """
        return float('%.5f' % (a + ((k - a) / (c + q * math.exp(1) ** (-b * x)) ** (1 / v))))

    def _get_function_average_consumption(self, function, node=None, resources=None):
        """

        :param function:
        :return: decimal average consumption
        """
        consumption_percentages = list()

        if node is None and resources is None:
            total_resources_amount = self.enop.get_total_resources_amount()
            nodes_number = len(self.enop.nodes)
        elif node is not None and resources is None:
            total_resources_amount = self.enop.available_resources[node]
            nodes_number = 1
        elif node is None and resources is not None:
            total_resources_amount = {r: sum([resources[n][r] for n in self.enop.nodes]) for r in self.enop.resources}
            nodes_number = len(self.enop.nodes)
        else:
            total_resources_amount = resources[node]
            nodes_number = 1

        average_node_resources = {}
        for resource in self.enop.resources:
            average_node_resources[resource] = total_resources_amount[resource]/nodes_number
        for resource in self.enop.resources:
            consumption = self.enop.get_consumption(function)[resource]
            total = average_node_resources[resource]
            if total == 0:
                return sys.maxsize
            consumption_percentages.append(consumption / total)

        av_decimal_consumption = sum(consumption_percentages) / float(len(consumption_percentages))
        return av_decimal_consumption
    '''
