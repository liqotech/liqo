class NoFunctionsLeft(Exception):
    def __init__(self, message):
        self.message = message
        # Call the base class constructor with the parameters it needs
        super(NoFunctionsLeft, self).__init__(message)

    def get_mess(self):
        return self.message


class SchedulingTimeout(Exception):
    def __init__(self, message):
        self.message = message
        # Call the base class constructor with the parameters it needs
        super(SchedulingTimeout, self).__init__(message)

    def get_mess(self):
        return self.message


class UtilityNotSupported(Exception):
    def __init__(self, message):
        self.message = message
        # Call the base class constructor with the parameters it needs
        super(UtilityNotSupported, self).__init__(message)

    def get_mess(self):
        return self.message
