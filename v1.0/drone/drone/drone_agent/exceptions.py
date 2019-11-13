class WrongConfigurationFile(Exception):
    def __init__(self, message):
        self.message = message
        # Call the base class constructor with the parameters it needs
        super(WrongConfigurationFile, self).__init__(message)

    def get_mess(self):
        return self.message
