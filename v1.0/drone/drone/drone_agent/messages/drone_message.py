from drone_agent.messages.message import Message


class DroneMessage(Message):

    def __init__(self, sender=None, winners=None, voting_data=None):
        """

        :param str sender:
        :param dict[str, dict[str, str]] winners:
        :param dict[str, dict[str, dict[str, union[int, str, float]]] voting_data:
        """
        super().__init__(sender)
        self.winners = winners
        self.voting_data = voting_data

    def to_dict(self):
        drone_message_dict = super(DroneMessage, self).to_dict()
        drone_message_dict["winners"] = self.winners
        drone_message_dict["voting-data"] = self.voting_data
        return drone_message_dict

    def parse_dict(self, drone_message_dict):
        super(DroneMessage, self).parse_dict(drone_message_dict)
        self.winners = drone_message_dict["winners"]
        self.voting_data = drone_message_dict["voting-data"]
