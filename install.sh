#!/bin/bash
set -e

function cleanup()
{
   set +e
   if [ -n "$TMPDIR" ]; then
     rm -rf "$TMPDIR"
   fi
}

function set_variable_from_command() {
    if [[ ($# -ne 3) ]];
    then
      echo "Internal Error - Wrong number of parameters"
      exit 1
    fi
    VAR_NAME=$1
    DEFAULT_COMMAND=$2
    if [ -z "${!VAR_NAME}" ]
    then
        result=$(bash -c "${!DEFAULT_COMMAND}") || {
          ret=$?; echo "$3 - Code: $ret"; return $ret; 
        }
        declare -gx "$VAR_NAME"=$result
    fi
    echo "[PRE-INSTALL]: $VAR_NAME is set to: ${!VAR_NAME}"
}

function print_help()
{
   echo "Arguments:"
   echo "   --help: print this help"
   echo "   --test: source file only for testing"
   echo "   -u, --uninstall: trigger uninstall workflow to remove LIQO from your cluster"
   echo "   --deleteCrd: remove all CRDs installed by LIQO"
   echo "This script is designed to install LIQO on your cluster. This script is configurable via environment variables:"
   echo "   POD_CIDR: the POD CIDR of your cluster (e.g.; 10.0.0.0/16). The script will try to detect it, but you can override this by having this variable already set"
   echo "   SERVICE_CIDR: the POD CIDR of your cluster (e.g.; 10.96.0.0/12) . The script will try to detect it, but you can override this by having this variable already set"
   echo "   GATEWAY_IP: the public IP that will be used by LIQO to establish the interconnection with other clusters"
   echo "   NAMESPACE: the namespace where LIQO control plane resources will be created"
}

function wait_and_approve_csr(){
   max_retry=10
   retry=0
   while [ "$retry" -lt "$max_retry" ]; do
     echo "[INSTALL]: Approving Admission/Mutating Webhook CSRs, $1"
     if kubectl get csr $1 > /dev/null; then
       kubectl certificate approve $1 || exit 1
       break
     fi
     echo "[INSTALL]: CSR not found... Retrying..."
     retry=$((retry+1))
     sleep 10
   done
   return 0
}

function set_gateway_node() {
   test=$(kubectl get no -l "liqonet.liqo.io/gateway=true" 2> /dev/null | wc -l)
   if [ $test == 0 ]; then
      node=$(kubectl get no -o jsonpath="{.items[-1].metadata.name}")
      kubectl label no $node liqonet.liqo.io/gateway=true > /dev/null
   fi
   address=$(kubectl get no -l "liqonet.liqo.io/gateway=true" -o jsonpath="{.items[0].status.addresses[0].address}")
   echo "$address"
}

function clone_repo() {
  if [ "$LIQO_SUFFIX" == "-ci" ] && [ ! -z "${LIQO_VERSION}" ]  ; then
    git clone "$URL" "$TMPDIR"/liqo
    cd "$TMPDIR"/liqo
    git checkout "$LIQO_VERSION" > /dev/null 2> /dev/null
    cd -
  else
    git clone "$URL" "$TMPDIR"/liqo --depth 1
  fi
}

function unjoin() {
  kubectl patch clusterconfig configuration -p '{"spec":{"advertisementConfig":{"outgoingConfig":{"enableBroadcaster":false}}}}' --type 'merge'

  retry=10
  while [ "$(kubectl get pods -n "$NAMESPACE" | grep -c broadcaster)" -gt 0 ]; do
    sleep 10
    retry=$((retry-1))
    if [ "$retry" -eq 0 ]; then
      echo "Max retries reached: unable to unpeer all clusters"
      exit 1
    fi
  done

  for id in $(kubectl get foreignclusters -o jsonpath="{.items[*].metadata.name}"); do
    kubectl patch foreignclusters "$id" -p '{"spec":{"join":false}}' --type 'merge'
  done
}

function deleteCrd() {
  echo "delete CRD"
  clone_repo
  kubectl delete -f "$TMPDIR"/liqo/deployments/liqo_chart/crds || true
  kubectl delete ns "$NAMESPACE"
}

function install() {
  clone_repo

  echo "[PRE-INSTALL]: Setting Gateway IP"
  GATEWAY_IP=$(set_gateway_node)
  echo "[PRE-INSTALL]: GATEWAY_IP is set to: $GATEWAY_IP"


  POD_CIDR_COMMAND='kubectl cluster-info dump | grep -m 1 -Po "(?<=--cluster-cidr=)[0-9.\/]+"'
  set_variable_from_command POD_CIDR POD_CIDR_COMMAND "[ERROR]: Unable to find POD_CIDR"
  SERVICE_CIDR_COMMAND='kubectl cluster-info dump | grep -m 1 -Po "(?<=--service-cluster-ip-range=)[0-9.\/]+"'
  set_variable_from_command SERVICE_CIDR SERVICE_CIDR_COMMAND "[ERROR]: Unable to find Service CIDR"
  NAMESPACE=${NAMESPACE:-$NAMESPACE_DEFAULT}
  LIQO_SUFFIX=${LIQO_SUFFIX:-$LIQO_SUFFIX_DEFAULT}
  LIQO_VERSION=${LIQO_VERSION:-$LIQO_VERSION_DEFAULT}



  #Wait for the installation to complete
  kubectl create ns $NAMESPACE || true
  $HELM_PATH dependency update $TMPDIR/liqo/deployments/liqo_chart
  $HELM_PATH install liqo -n $NAMESPACE $TMPDIR/liqo/deployments/liqo_chart --set podCIDR=$POD_CIDR --set serviceCIDR=$SERVICE_CIDR \
    --set gatewayIP=$GATEWAY_IP --set global.suffix="$LIQO_SUFFIX" --set global.version="$LIQO_VERSION"
  echo "[INSTALL]: Installing LIQO on your cluster..."
  sleep 30

  # Approve CSRs

  wait_and_approve_csr "peering-request-operator.$NAMESPACE"
  wait_and_approve_csr "mutatepodtoleration.$NAMESPACE"
}

function uninstall() {
  NAMESPACE=${NAMESPACE:-$NAMESPACE_DEFAULT}

  echo "[PRE-UNINSTALL]: Unpeering clusters"
  unjoin

  echo "[UNINSTALL]: Uninstalling LIQO on your cluster..."
  $HELM_PATH uninstall liqo -n "$NAMESPACE"
  sleep 30

  kubectl delete MutatingWebhookConfiguration mutatepodtoleration || true
  kubectl delete ValidatingWebhookConfiguration peering-request-operator || true

  kubectl delete csr "peering-request-operator.$NAMESPACE" > /dev/null 2> /dev/null || true
  kubectl delete csr "mutatepodtoleration.$NAMESPACE" > /dev/null 2> /dev/null || true
}

if [[ ($# -eq 1 && $1 == '--help') ]];
then
   print_help
   exit 0
# The next line is required to easily unit-test the functions previously declared
elif [[ $# -eq 1 && $0 == '/opt/bats/libexec/bats-core/bats-exec-test' ]]; then
  echo "Testing..."
  return 0
else
  while [ "$1" != "" ]; do
    case $1 in
      -u | --uninstall )
        UNINSTALL=1
        ;;
      --deleteCrd )
        DELETE_CRD=1
        ;;
      * )
        echo "ERROR: Illegal parameters"
        print_help
        exit 1
    esac
    shift
  done
fi


trap cleanup EXIT
URL=https://github.com/LiqoTech/liqo.git
HELM_VERSION=v3.2.3
HELM_ARCHIVE=helm-${HELM_VERSION}-linux-amd64.tar.gz
HELM_URL=https://get.helm.sh/$HELM_ARCHIVE
NAMESPACE_DEFAULT="liqo"
# The following variable are used a default value to select the images when installing LIQO.
# When installing a non released version:
# - Export LIQO_SUFFIX="-ci" if the commit is not on the master branch
# - Export LIQO_VERSION to the id of your commit
LIQO_VERSION_DEFAULT="latest"
LIQO_SUFFIX_DEFAULT=""

# Necessary Commands
commands="curl kubectl"

echo "[PRE-CHECK]: Checking all pre-requisites are met"
for val in $commands; do
    if command -v $val > /dev/null; then
      echo "[PRE-CHECK]: $val correctly found"
    else
      echo "[PRE-CHECK] [FATAL] : $val not found. Exiting"
      exit 1
    fi
done

TMPDIR=$(mktemp -d)
mkdir -p $TMPDIR/bin/
echo "[PRE-CHECK] [HELM]: Downloading Helm $HELM_VERSION"
curl --fail -L ${HELM_URL} | tar zxf - --directory="$TMPDIR/bin/" --wildcards '*/helm' --strip 1
HELM_PATH="$TMPDIR/bin/helm"


echo "[PRE-CHECK]: Collecting installation variables. The installer will retrieve installation parameters from your
 Kubernetes cluster. Feel free to override them, by launching it with those environment variables set in advance."
if [ ! -z "$KUBECONFIG" ]
then
  echo "[PRE-CHECK]: Kubeconfig variable is set to: $KUBECONFIG"
else
  echo "[PRE-CHECK]: Kubeconfig variable is not set. Kubectl will use: ~/.kube/config"
fi

if [ -n "$UNINSTALL" ]; then
  uninstall
fi

if [ -n "$DELETE_CRD" ]; then
  deleteCrd
  # if we delete CRDs we will not proceed to installation
  # this handles the call with --deleteCRD flag only
  exit 0
fi

if [ -z "$UNINSTALL" ]; then
  install
fi
