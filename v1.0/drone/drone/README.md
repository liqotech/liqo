# DRONE v1.0 README
Distributed Resources Offload at Network Edge

Updated May 22, 2019


#### @Copyright
DRONE is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version.
DRONE is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public License for more details.
You should have received a copy of the GNU General Public License along with DRAGON. If not, see <http://www.gnu.org/licenses/>.


#### About the project

This repository provides a distributed algorithm that enables multiple compute edge nodes to share resources optimizing the deployment of concurrent applications. 


#### Repository Structure

The repository tree contains: 


* [README.md](README.md)  
    --this file  
* [LICENSE](LICENSE)  
    --GPLv3 license  
* [main.py](main.py)  
    --main instance executable  
* [config/](config)  
    --configuration files  
* [drone\_agent/](drone_agent)  
    --DRAGON agent source files  
* [resource\_offloading/](resource_offloading)  
    --implementation of the resources offloading problem  
* [scripts/](scripts)  
    --useful scripts related to the project  
* [tests/](tests)  
    --validation purpose scripts  

### Fetch

Download the source code by cloning the repository, as well as any submodule:

    $ git clone https://github.com/netgroup-polito/drone
    $ cd drone
    $ git submodule update --init

### Configuration

[config/default\_config.ini](config/default_config.ini) -- main configuration file  
[config/config.py](config/config.py) -- agent and instance configuration  
[config/enop\_instance.json](config/enop_instance.json) -- resource offloading problem instance

Please create a copy of the main configuration file, and edit your configuration according with your own setup:

    $ cp config/default_config.ini config/config.ini

### Install

This project requires python 3.6 and has been tested on Linux debian (testing) with kernel 4.16.0-2-amd64.

Some additional python packages are required:

    # apt install python3-pip
    # pip3 install -r requirements.txt
    
#### Rabbit Setup

Inter agent communication is implemented over the RabbitMQ Broker. To install it use the following command: 

    # apt install rabbitmq-server

This projects uses the federation feature to enable inter-broker exchanges. 
For this reason, the RabbitMQ server must be configured with a User, Policy and Federation Upstream.

First of all, install the Federation Plugin:

    # /usr/sbin/rabbitmq-plugins enable rabbitmq_federation rabbitmq_federation_management

then restart RabbitMQ.

In order to setup the federation between the different RabbitMQ servers, you can:
* Follow the manual installation described in [README_RABBITMQ.md](README_RABBIT.md) 
* Run the script [scripts/rabbit_setup.py](scripts/rabbit_setup.py) as superuser:

        # python3 -m script.rabbit_setup

The script creates a new user, a Policy and all Federation Upstreams.

To indicate the peer brokers to be federated, please modify the peer list in the script.
For example, on rabbit0 Broker, to federate to rabbit1 and rabbit2 the list will be:

    peers = [["rabbit1", "10.0.0.1"],["rabbit2","10.0.0.2"]]

Where "rabbit1" is the name of federation upstream towards the rabbit1 Broker, and 10.0.0.1 its IP address.

You can modify the [config/config.ini](config/config.ini) file with the chosen credentials and federation parameters.

    
    
### Run

Make sure rabbitmq is running:

    # service rabbitmq-server start

The [main.py](main.py) script runs a single instance of the DRAGON agent. To run it, use the following command from the project root directory:

    $ python3 main.py {agent-name} [-d {configuration-file}] [-p]
    
where:

- ***agent-name***: is a name to identify the agent;
- ***services***: is a list of parameters, namely the names of services for which the agent will attempt to obtain resources (see [config/rap\_instance.json]()).
- ***configuration-file***: is the path of the configuration file to use (default is [config/default_config.ini](config/default_config.ini)).
- ***-p***: if given, the agent starts in persistent mode, that is, it will not exit after the first agreement is reached, but will listen for any change that requires a new agreement.


#### Testing

The [tests/](tests) folder also contains a script that automatically runs multiple agents at the same time. 
Please modify [config/default_config.ini](config/default_config.ini) as desired before to run it, so to specify instance parameters, then use:

    $ python3 -m tests.test_script
    
The number of agent specified in the configuration file will be run and the script will wait for convergence.
At the end of the execution, the log file of each agent will be available in the main folder, while details on the resulting assignments will be stored on the (generated) [results](results) folder.

##### Tests on multiple remote hosts
 
An alternative script allows you to perform tests while running agents on remote hosts. 
Since this requires to setup ssh connections with the remote hosts, please install the ssh server on each of them:
 
    # apt install openssh-server
 
then please setup your ssh public key to be accepted on every target host.

You may need to increase the limit of ssh connections accepted on each host, by modifying the 'MaxStartups' parameter in the sshd configuration file:

    # nano /etc/ssh/sshd_config
    
Then, assuming you want to allow up to 50 connections, change the 'MaxStartups' line as follows:

    MaxStartups 50:30:100
    
Close and save the file, then restart the ssh daemon:

    # service sshd restart

Make sure rabbitmq is running on every host thorugh federated setup (see [https://www.rabbitmq.com/federation.html]()).

The [tests/test_script_ssh.py](tests/test_script_ssh.py) script can be setup specifying the list of remote hosts, the username to be used for the ssh connections and the remote path where dragon is located. Please modify these values in the first lines of the script according to your setup.

Analogously to the local test script, you can run the remote test using: 
   
    $ python3 -m tests.test_script_ssh
    
The script will automatically copy the local configuration to the remote hosts, and a drone agent will be run on each of them (agents that are neighbors in the topology file will be likely deployed on near hosts). All output and log files will be fetched and results displayed locally.
