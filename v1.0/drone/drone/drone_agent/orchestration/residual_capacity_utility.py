from drone_agent.orchestration.private_utility_interface import PrivateUtilityInterface


class ResidualCapacityUtility(PrivateUtilityInterface):
    """
    The residual capacity (stress profile) on the hosting node is used as marginal utility for each component.
    """

    def marginal_utility(self, offloading_bundle, app, component, implementation):
        """
        Returns a "stress profile" utility, proportional to the residual capacity on the node if the component is added
        to the local bundle.
        :param offloading_bundle:
        :param app:
        :param component:
        :param implementation:
        :return: the marginal utility as normalized residual capacity
        """
        # compute stress on node
        residual_resources = self.enop.available_resources[self.node_name]
        # remove resources for already scheduled components
        for offloaded_app, offloaded_component, offloaded_implementation in offloading_bundle:
            residual_resources = self.enop.sub_resources(residual_resources,
                                                         self.enop.get_consumption(offloaded_implementation))
        # remove resources for the new candidate component
        residual_resources = self.enop.sub_resources(residual_resources, self.enop.get_consumption(implementation))
        residual_norm = self.enop.norm(self.node_name, residual_resources)
        marginal_utility = int(round(residual_norm*100))

        return marginal_utility

    def to_be_released(self, offloading_bundle, removing_items):
        """
        Utilities of elements in bundle may depends from components taken before them,
        then the utility of all elements after the first one that has been lost, potentially changed.
        :param offloading_bundle:
        :param removing_items: the tuple is (app, component)
        :return list: the list of components whose utility is not valid anymore
        """
        if not removing_items or not offloading_bundle:
            return list()
        stripped_offloading_bundle = [(a, c) for a, c, f in offloading_bundle]
        # identify the lost component that was inserted first
        first_lost_component = sorted(removing_items, key=lambda c: stripped_offloading_bundle.index(c))[0]
        # index of the first one
        first_lost_component_index = stripped_offloading_bundle.index(first_lost_component)
        # release all the components inserted after it, leave the others
        to_release = offloading_bundle[first_lost_component_index:]
        return to_release
