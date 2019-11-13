from enum import Enum

from drone_agent.messages.exceptions import InvalidMessageError
from drone_agent.messages.message import Message


class AppAdvMessage(Message):

    def __init__(self, sender=None, app_name=None, adv_type=None, components=None):
        """

        :param str sender:
        :param str app_name:
        :param Enum adv_type:
        :param list of AppAdvMessage.Component components:
        """
        super().__init__(sender)
        self.app_name = app_name
        self.base_node = None
        self.adv_type = adv_type
        self.components = components

    def to_dict(self):
        app_adv_message_dict = super(AppAdvMessage, self).to_dict()
        app_adv_message_dict["app_name"] = self.app_name
        app_adv_message_dict["type"] = self.adv_type.name
        if self.adv_type == AppAdvMessage.AdvertisementType.ADD or self.adv_type == AppAdvMessage.AdvertisementType.MOD:
            app_adv_message_dict["components"] = [component.to_dict() for component in self.components]
        return app_adv_message_dict

    def parse_dict(self, app_adv_message_dict):
        if "sender" not in app_adv_message_dict:
            app_adv_message_dict["sender"] = None
        try:
            super(AppAdvMessage, self).parse_dict(app_adv_message_dict)
            self.app_name = app_adv_message_dict["app_name"]
            self.adv_type = self.AdvertisementType[app_adv_message_dict["type"]]
            if self.adv_type == AppAdvMessage.AdvertisementType.ADD \
                    or self.adv_type == AppAdvMessage.AdvertisementType.MOD:
                self.components = [AppAdvMessage.Component(component_dict=component_dict)
                                   for component_dict in app_adv_message_dict["components"]]
                if not self.components:
                    raise InvalidMessageError("Received message is not valid: components list is empty.")
        except KeyError as err:
            raise InvalidMessageError("Received message is not valid: missing key '{}'.".format(err))
        except TypeError as err:
            raise InvalidMessageError("Received message is not valid: '{}'.".format(err))

    class AdvertisementType(Enum):
        ADD = 1
        DEL = 2
        MOD = 3

    class Component:

        def __init__(self, component_dict=None):

            self.name = None
            self.function = None
            self.nodes_blacklist = None
            self.nodes_whitelist = None
            if component_dict is not None:
                self.parse_dict(component_dict)

        def to_dict(self):
            app_description_dict = dict()
            app_description_dict["name"] = self.name
            if self.function is not None:
                app_description_dict["function"] = self.function.to_dict
            app_description_dict["nodes-blacklist"] = self.nodes_blacklist
            app_description_dict["nodes-whitelist"] = self.nodes_whitelist
            return app_description_dict

        def parse_dict(self, app_description_dict):
            self.name = app_description_dict["name"]
            if "function" in app_description_dict.keys():
                self.function = AppAdvMessage.Function(function_dict=app_description_dict["function"])
            self.nodes_blacklist = app_description_dict["nodes-blacklist"]
            self.nodes_whitelist = app_description_dict["nodes-whitelist"]

    class Function:

        def __init__(self, function_dict=None):
            self.image = None
            self.consumption = dict()
            if function_dict is not None:
                self.parse_dict(function_dict)

        def to_dict(self):
            function_dict = dict()
            function_dict["image"] = self.image
            function_dict["resources"] = self.consumption
            return function_dict

        def parse_dict(self, function_dict):
            self.image = function_dict["image"]
            self.consumption = function_dict["resources"]
