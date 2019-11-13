import time

from drone_agent.messages.message import Message


class ResourceMessage(Message):

    def __init__(self, sender=None, node_resources=None):
        """

        :param str sender:
        :param dict[str, int] node_resources:
        """
        super().__init__(sender)
        self.node_resources = node_resources
        self.timestamp = time.time()

    def to_dict(self):
        solution_message_dict = super(ResourceMessage, self).to_dict()
        solution_message_dict["node_resources"] = self.node_resources
        return solution_message_dict

    def parse_dict(self, solution_message_dict):
        super(ResourceMessage, self).parse_dict(solution_message_dict)
        self.node_resources = solution_message_dict["node_resources"]
