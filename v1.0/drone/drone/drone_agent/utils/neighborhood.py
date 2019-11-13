import hashlib
import json
import time


class NeighborhoodDetector:

    def __init__(self, nodes, base_node, load_neighborhood, neighbor_probability=None, topology_file=None,
                 stable_connections=False):
        """

        :param list of str nodes:
        :param str base_node:
        :param bool load_neighborhood:
        :param int neighbor_probability:
        :param str topology_file:
        :param bool stable_connections: if True, neighborhood is fixed
        """
        self.nodes = nodes
        self.base_node = base_node
        self.neighbor_probability = neighbor_probability
        self.stable_connections = stable_connections
        if load_neighborhood:
            with open(topology_file, "r") as f:
                topology = json.loads(f.read())
            self.neighborhood = [node for node in self.nodes
                                 if node != self.base_node
                                 and self._topology_check_neighbors(self.base_node, node, topology)]
        else:
            self.neighborhood = [node for node in self.nodes
                                 if node != self.base_node
                                 and self._pseudo_random_check_neighbors(self.base_node, node)]

    def get_neighborhood(self):
        """
        Return the list of static neighbors.
        :return:
        """
        return self.neighborhood

    def get_current_neighborhood(self):
        """
        Return the list of current connected neighbors.
        :return:
        """
        if self.stable_connections:
            return self.neighborhood
        else:
            return [node for node in self.neighborhood if self._pseudo_random_check_connection(self.base_node, node)]

    def _pseudo_random_check_neighbors(self, node1, node2):
        """

        :param node1:
        :param node2:
        :return:
        """
        # establish if the given sdos are physically one-hop-neighbors
        nodes_digest = int(hashlib.sha256(("1" + str(sorted([node1, node2]))).encode()).hexdigest(), 16)
        return int(str(nodes_digest)[-2:]) < self.neighbor_probability
        # return int(bin(sdos_digest)[-1:]) == 0

    def _topology_check_neighbors(self, node1, node2, topology):
        """

        :param node1:
        :param node2:
        :param topology:
        :return:
        """
        # establish if the given sdos are physically one-hop-neighbors
        return node2 in self.nodes and node2 in topology[node1]

    def _pseudo_random_check_connection(self, node1, node2):
        """

        :param node1:
        :param node2:
        :return bool:
        """
        if node2 not in self.neighborhood:
            return False

        if not self.stable_connections:
            # establish if at the given time the two neighbors has connectivity (prob 0.75)
            # may change every 10 seconds
            time_token = int(time.time()/10)
            nodes_time_digest = int(hashlib.sha256((str(sorted([node1, node2])) + str(time_token)).encode())
                                    .hexdigest(), 16)
            return int(bin(nodes_time_digest)[-2:]) != 0

        return True

    def _check_neighbors(self, node1, node2):
        """

        :param node1:
        :param node2:
        :return:
        """
        pass
