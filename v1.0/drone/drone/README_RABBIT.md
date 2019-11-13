## RabbitMQ Federation manual install

Updated May 18, 2019

To use Federation, RabbitMQ Broker must be configured with a User, Policy and Federation Upstreams.

First of all install Federation Plugin:

    $ sudo /usr/sbin/rabbitmq-plugins enable rabbitmq_federation rabbitmq_federation_management

then restart RabbitMQ.


In order to setup the federation between the different RabbitMQ servers, you will need to a bit of command
line work on each of the servers.

Setup a new user (e.g., dragon:dragon) and modify your [config/config.ini](config/config.ini) file with the chosen credentials:

    $ sudo rabbitmqctl add_user drone drone
    $ sudo rabbitmqctl set_user_tags drone administrator
    $ sudo rabbitmqctl set_permissions -p / drone ".*" ".*" ".*"

Then for example, on rabbit0 Broker, federate to 1 and 2:

    $ sudo rabbitmqctl set_parameter federation-upstream rabbit1 '{"uri":"amqp://dragon:dragon@10.0.0.1"}'
    $ sudo rabbitmqctl set_parameter federation-upstream rabbit2 '{"uri":"amqp://dragon:dragon@10.0.0.2"}'
    $ sudo rabbitmqctl set_parameter federation-upstream-set drone '[{"upstream":"rabbit1"},{"upstream":"rabbit2"}]'
    $ sudo rabbitmqctl set_policy --apply-to exchanges federate-drone drone-exchange '{"federation-upstream-set":"drone"}'

This does the following:

* Define two upstream nodes (rabbit1 and rabbit2) and assign them the correct addresses. Then build an upstream-set
  called 'drone' that contains those two nodes
* Create a policy (called 'federate-drone') that selects exchange 'drone-exchange' and federate it to the
  upstream-set 'drone'

You will need to run a similar set of commands on each other rabbit instance to connect them as well.

All this can be done from the RabbitMQ Web UI by activating the plugin 'rabbitmq_management':

    $ sudo /usr/sbin/rabbitmq-plugins enable rabbitmq_management

The RabbitMQ Web UI is available on http://localhost:15672.
