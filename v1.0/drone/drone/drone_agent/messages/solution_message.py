import time

from drone_agent.messages.message import Message


class SolutionMessage(Message):

    def __init__(self, sender=None, local_offloading=None, overall_offloading=None):
        """

        :param str sender:
        :param list of SolutionMessage.OffloadedComponent local_offloading:
        """
        super().__init__(sender)
        self.local_offloading = local_offloading
        self.overall_offloading = overall_offloading

    def to_dict(self):
        solution_message_dict = super(SolutionMessage, self).to_dict()
        solution_message_dict["local-offloading"] = [oc.to_dict() for oc in self.local_offloading]
        solution_message_dict["overall-offloading"] = self.overall_offloading
        return solution_message_dict

    def parse_dict(self, solution_message_dict):
        super(SolutionMessage, self).parse_dict(solution_message_dict)
        self.local_offloading = [SolutionMessage.OffloadedComponent(oc_dict)
                                 for oc_dict in solution_message_dict["local-offloading"]]
        self.overall_offloading = solution_message_dict["overall-offloading"]

    class OffloadedComponent:

        def __init__(self, name=None, application=None, function=None, offloaded_component_dict=None):
            """

            :param str name:
            :param str application:
            :param SolutionMessage.Function function:
            :param dict offloaded_component_dict:
            """
            self.name = name
            self.application = application
            self.function = function
            if offloaded_component_dict is not None:
                self.parse_dict(offloaded_component_dict)

        def to_dict(self):
            offloaded_component_dict = dict()
            offloaded_component_dict["name"] = self.name
            offloaded_component_dict["app_name"] = self.application
            offloaded_component_dict["function"] = self.function.to_dict()
            return offloaded_component_dict

        def parse_dict(self, offloaded_component_dict):
            self.name = offloaded_component_dict["name"]
            self.application = offloaded_component_dict["app_name"]
            self.function = SolutionMessage.Function(offloaded_component_dict["function"])

    class Function:

        def __init__(self, name=None, consumption=None, function_dict=None):
            self.name = name
            self.consumption = consumption
            if function_dict is not None:
                self.parse_dict(function_dict)

        def to_dict(self):
            function_dict = dict()
            function_dict["name"] = self.name
            function_dict["resources"] = self.consumption
            return function_dict

        def parse_dict(self, function_dict):
            self.name = function_dict["name"]
            self.consumption = function_dict["resources"]
