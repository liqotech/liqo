#!/bin/bash
set -e

function cleanup()
{
   set +e
}

set_variable_from_command() {    
    if [[ ($# -ne 2) ]];
    then
      echo "Internal Error - Wrong number of parameters"
      exit 1
    fi
    VAR_NAME=$1
    DEFAULT_COMMAND=$2
    if [ -z "${!VAR_NAME}" ]
    then
        result=$(bash -c "${!DEFAULT_COMMAND}") || return $?
        declare -gx "$VAR_NAME"=$result
    fi
    echo "[PRE-INSTALL]: $VAR_NAME is set to: ${!VAR_NAME}"
}

print_help()
{
   echo "Arguments:"
   echo "   --help: print this help"
   echo "   --test: source file only for testing"
   echo "This script is designed to install LIQO on your cluster. This script is configurable via environment variables:"
   echo "   POD_CIDR: the POD CIDR of your cluster (e.g.; 10.0.0.0/16). The script will try to detect it, but you can override this by having this variable already set"
   echo "   SERVICE_CIDR: the POD CIDR of your cluster (e.g.; 10.96.0.0/12) . The script will try to detect it, but you can override thisthis by having this variable already set"
   echo "   GATEWAY_PRIVATE_IP: the IP used by the cluster inside the cluster-to-cluster interconnection (e.g.; 192.168.1.1)"
   echo "   GATEWAY_IP: the public IP that will be used by LIQO to establish the interconnection with other clusters"
}


if [[ ($# -eq 1 && $1 == '--help') ]];
then
     print_help
     exit 0
# The next line is required to easily unit-test the functions previously declared
elif [[ $# -eq 1 && $0 == '/opt/bats/libexec/bats-core/bats-exec-test' ]]
then
     echo "Testing..."
     return 0
elif [[ $# -ge 1 ]]
then
     echo "ERROR: Illegal parameters"
     print_help
     exit 1
fi


trap cleanup EXIT
URL=https://github.com/LiqoTech/liqo.git
HELM_VERSION=v3.2.3
HELM_ARCHIVE=helm-${HELM_VERSION}-linux-amd64.tar.gz
HELM_URL=https://get.helm.sh/$HELM_ARCHIVE
DEFAULT_GATEWAY_PRIVATE_IP=192.168.1.1

# Necessary Commands
commands="curl kubectl"

echo "[PRE-INSTALL]: Checking all pre-requisites are met"
for val in $commands; do
    if command -v $val > /dev/null; then
      echo "[PRE-INSTALL]: $val correctly found"
    else
      echo "[PRE-INSTALL] [FATAL] : $val not found. Exiting"
      exit 1
    fi
done

TMPDIR=$(mktemp -d)
mkdir -p $TMPDIR/bin/
echo "[PRE-INSTALL] [HELM] Checking HELM installation..."
echo "[PRE-INSTALL] [HELM]: Downloading Helm $HELM_VERSION"
curl --fail -L ${HELM_URL} | tar zxf - --directory="$TMPDIR/bin/" --wildcards '*/helm' --strip 1
git clone "$URL" $TMPDIR/liqo --depth 1

echo "[PRE-INSTALL]: Collecting installation variables. The installer will retrieve installation parameters from your
 Kubernetes cluster. Feel free to override them, by launching it with those environment variables set in advance."
if [ ! -z "$KUBECONFIG" ]
then
  echo "[PRE-INSTALL]: Kubeconfig variable is set to: $KUBECONFIG"
else
  echo "[PRE-INSTALL]: Kubeconfig variable is not set. Kubectl will use: ~/.kube/config"
fi

POD_CIDR_COMMAND='kubectl cluster-info dump | grep -m 1 -Po "(?<=--cluster-cidr=)[0-9.\/]+"'
set_variable_from_command POD_CIDR POD_CIDR_COMMAND
SERVICE_CIDR_COMMAND='kubectl cluster-info dump | grep -m 1 -Po "(?<=--service-cluster-ip-range=)[0-9.\/]+"'
set_variable_from_command SERVICE_CIDR SERVICE_CIDR_COMMAND 
GATEWAY_IP_COMMAND='kubectl get no -l "liqonet.liqo.io/gateway=true" -o jsonpath="{.items[0].status.addresses[0].address}"'
set_variable_from_command GATEWAY_IP GATEWAY_IP_COMMAND
GATEWAY_PRIVATE_IP_COMMAND="echo $DEFAULT_GATEWAY_PRIVATE_IP"
set_variable_from_command GATEWAY_PRIVATE_IP GATEWAY_PRIVATE_IP_COMMAND

TO_EXECUTE="$TMPDIR/bin/helm install liqo -n liqo ./ --set podCIDR=$POD_CIDR --set serviceCIDR=$SERVICE_CIDR --set gatewayPrivateIP=$GATEWAY_PRIVATE_IP --set gatewayIP=$GATEWAY_IP"
read -p "Are you sure to deploy LIQO with this configuration? (y/N) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]
then
    cd $TMPDIR/liqo
    $TO_EXECUTE
fi
