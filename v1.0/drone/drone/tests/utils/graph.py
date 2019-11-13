from collections import OrderedDict

import networkx as nx


class Graph:
    """
    Here 'node' is a node of the graph, i.e., an sdo.
    """

    def __init__(self, topology, n):

        topology = OrderedDict(topology)
        topology = {node: topology[node] for i, node in enumerate(topology) if i < n}
        for node in topology:
            topology[node] = [neighbor for neighbor in topology[node] if neighbor in topology.keys()]

        self.topology = topology
        self.graph = nx.Graph()

        self.graph.add_edges_from([(node1, node2)
                                   for node1 in self.topology.keys()
                                  for node2 in self.topology[node1]])

        if len(self.topology.keys()) == 1:
            self.graph.add_nodes_from(self.topology.keys())

    def print_topology(self):
        print("-------- Topology ---------")
        for node, neighborhood in self.topology.items():
            print(node + " -> " + str(neighborhood))
        print("---------------------------")
