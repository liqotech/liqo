class MultipleTimedConnectionNotSupportedError(Exception):
    def __init__(self, message):
        self.message = message
        # Call the base class constructor with the parameters it needs
        super(MultipleTimedConnectionNotSupportedError, self).__init__(self.message)

    def get_mess(self):
        return self.message


class ConnectionNotFoundError(Exception):
    def __init__(self, connection_id):
        self.message = "Connection '{}' not found".format(connection_id)
        # Call the base class constructor with the parameters it needs
        super(ConnectionNotFoundError, self).__init__(self.message)

    def get_mess(self):
        return self.message
