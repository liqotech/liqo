import time
from abc import ABCMeta, abstractmethod


class Message:
    """
    Generic definitions for message classes. This abstract version has no body.
    """
    def __init__(self, sender=None):
        """

        :param str sender:
        """
        self.sender = sender
        self.timestamp = time.time()

    def to_dict(self):
        drone_message_dict = dict()
        drone_message_dict["sender"] = self.sender
        drone_message_dict["timestamp"] = self.timestamp
        return drone_message_dict

    def parse_dict(self, drone_message_dict):
        self.sender = drone_message_dict["sender"]
        self.timestamp = drone_message_dict["timestamp"]
