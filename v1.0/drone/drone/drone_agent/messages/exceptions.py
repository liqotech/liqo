class InvalidMessageError(Exception):
    def __init__(self, message):
        self.message = message
        # Call the base class constructor with the parameters it needs
        super(InvalidMessageError, self).__init__(self.message)

    def get_mess(self):
        return self.message
