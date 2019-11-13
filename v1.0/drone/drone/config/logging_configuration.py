import logging
import colorlog


class LogConfiguration:

    VERBOSE = 15
    IMPORTANT = 25

    def __init__(self, log_level, log_file=None):
        """

        :param log_level:
        :param log_file:
        """
        self.log_level = log_level
        self.log_file = log_file

    def configure_log(self):

        logging.addLevelName(self.VERBOSE, "VERBOSE")
        logging.addLevelName(self.IMPORTANT, "IMPORTANT")
        log_colors = colorlog.default_log_colors
        log_colors["INFO"] = "cyan"
        log_colors["VERBOSE"] = "black"
        log_colors["IMPORTANT"] = "green"
        message_format = "%(log_color)s%(asctime)s.%(msecs)03d | %(levelname)10s | [%(funcName)15s] %(message)s - " \
                         "%(filename)s:%(lineno)s"
        formatter = colorlog.ColoredFormatter(
            message_format,
            datefmt="%d/%m/%Y %H:%M:%S",
            log_colors=log_colors,
            secondary_log_colors={},
            style='%'
        )
        if self.log_level == "DEBUG":
            log_level = logging.DEBUG
            logging.getLogger("pika").setLevel(logging.INFO)
        elif self.log_level == "VERBOSE":
            log_level = self.VERBOSE
            logging.getLogger("pika").setLevel(logging.INFO)
        elif self.log_level == "INFO":
            log_level = logging.INFO
            logging.getLogger("pika").setLevel(logging.INFO)
        elif self.log_level == "IMPORTANT":
            log_level = self.IMPORTANT
            logging.getLogger("pika").setLevel(logging.WARNING)
        elif self.log_level == "WARNING":
            log_level = logging.WARNING
            logging.getLogger("pika").setLevel(logging.WARNING)
        else:
            log_level = logging.ERROR
            logging.getLogger("pika").setLevel(logging.ERROR)
        stream_handler = logging.StreamHandler()
        stream_handler.setFormatter(formatter)

        if self.log_file is not None:
            file_handler = logging.FileHandler(self.log_file, mode='w')
            message_format = message_format.replace("%(log_color)", "")
            logging.basicConfig(level=log_level,
                                format=message_format,
                                datefmt='%d/%m/%Y %H:%M:%S',
                                handlers=[file_handler])
        else:
            logging.basicConfig(level=log_level,
                                datefmt='%d/%m/%Y %H:%M:%S',
                                handlers=[stream_handler])
