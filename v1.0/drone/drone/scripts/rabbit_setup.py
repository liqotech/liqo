import os

from config.config import Configuration


def federation(_username, _password, _peers, _set, _policy, _exchange):

    for peer in _peers:
        federation_upstream = 'rabbitmqctl set_parameter federation-upstream ' + str(peer[0]) + ' \'{"uri":"amqp://'\
                              + _username + ':' + _password + '@' + peer[1] + '"}\''
        cmd = os.popen(federation_upstream).read()
        print(cmd)

    federation_upstream_set = 'rabbitmqctl set_parameter federation-upstream-set ' + _set + ' \'['
    for peer in _peers:
        federation_upstream_set = federation_upstream_set + '{"upstream":"' + str(peer[0]) + '"},'
    federation_upstream_set = federation_upstream_set + ']\''
    cmd = os.popen(federation_upstream_set).read()
    print(cmd)

    set_policy = 'rabbitmqctl set_policy --apply-to exchanges ' + _policy + ' "' + _exchange +\
                 '" \'{"federation-upstream-set":"' + _set + '"}\''
    cmd = os.popen(set_policy).read()
    print(cmd)


def new_rabbit_user(_username, _password):

    add_user_command = 'rabbitmqctl add_user ' + _username + ' ' + _password
    cmd = os.popen(add_user_command).read()
    print(cmd)
    set_user_tag = 'rabbitmqctl set_user_tags ' + _username + ' administrator'
    cmd = os.popen(set_user_tag).read()
    print(cmd)
    set_permissions = 'rabbitmqctl set_permissions -p / ' + _username + ' ".*" ".*" ".*"'
    cmd = os.popen(set_permissions).read()
    print(cmd)


if __name__ == '__main__':

    # [ Configuration ]
    CONF_FILE = 'config/config.ini'
    configuration = Configuration(CONF_FILE)

    peers = [["rabbit1", "10.0.0.1"], ["rabbit2", "10.0.0.2"]]

    new_rabbit_user(configuration.USERNAME, configuration.PASSWORD)
    federation(configuration.USERNAME, configuration.PASSWORD, peers,
               configuration.SET_NAME,
               configuration.POLICY_NAME,
               configuration.EXCHANGE_NAME)
