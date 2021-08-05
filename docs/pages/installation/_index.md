---
title: Installation 
weight: 2
---

### Pre-Installation

Liqo can be used with different topologies and scenarios. This impacts several installation parameters you will configure (e.g., API Server, Authentication).
Before installing Liqo, you should:
* Provision the clusters you would like to use with Liqo. If you need some advice about how to provision clusters on major providers, we have provided [here](./platforms/) some tips.
* Have a look to the [scenarios page](./pre-install) presents some common patterns used to expose and interconnect clusters.

### Quick Install

#### Pre-Requirements

To install Liqo, you have to install the following dependencies:

* [Helm 3](https://helm.sh/docs/intro/install/)
* [jq](https://stedolan.github.io/jq/download/)

To install Liqo on your cluster, you should know:

* **PodCIDR**, the address space of IPs assigned to Pods
* **ServiceCIDR**:  the address space of IPs assigned to ClusterIP services

{{% notice note %}}
Liqo only supports Kubernetes >= 1.19.0.
{{% /notice %}}

Depending on the provider, you have different way to retrieve those parameters. For more information, you can check in the following subsections:

{{%expand " Kubeadm" %}}
To retrieve PodCIDR and ServiceCIDR in a Kubeadm cluster, you can just extract it from the kube-controller-manager spec:

```bash
POD_CIDR=$(kubectl get pods --namespace kube-system --selector component=kube-controller-manager --output jsonpath="{.items[*].spec.containers[*].command}" 2>/dev/null | grep -Po --max-count=1 "(?<=--cluster-cidr=)[0-9.\/]+")
SERVICE_CIDR=$(kubectl get pods --namespace kube-system --selector component=kube-controller-manager --output jsonpath="{.items[*].spec.containers[*].command}" 2>/dev/null | grep -Po --max-count=1 "(?<=--service-cluster-ip-range=)[0-9.\/]+")
echo "POD CIDR: $POD_CIDR"
echo "SERVICE CIDR: $SERVICE_CIDR"
```
{{% /expand%}}

{{%expand " AWS Elastic Kubernetes Service (EKS)" %}}
Create an AWS IAM user for Liqo, it will use it to grant access on the required resource to the remote clusters:
```bash
LIQO_USER_NAME=liqo
LIQO_POLICY_NAME=liqo
LIQO_CLUSTER_REGION=eu-west-1
LIQO_CLUSTER_NAME=liqo-cluster
```

```bash
# create the liqo user
aws iam create-user --user-name "$LIQO_USER_NAME" > /dev/null

# create access keys for the user
aws iam create-access-key --user-name "$LIQO_USER_NAME" > iamKeys.json

# get the accessKeyId
cat iamKeys.json | jq -r '.AccessKey.AccessKeyId'

# get the secretAccessKey
cat iamKeys.json | jq -r '.AccessKey.SecretAccessKey'
```

{{% notice warning %}}
To ensure the security of your account, the secret access key is accessible only during key and user creation. You must save the key (for example, in a text file) if you want to be able to access it again. If a secret key is lost, you can delete the access keys for the associated user and then create new keys.
{{% /notice %}}

Now, create the policy required by the Liqo user and bind it
```bash
# create the policy file
cat > policy << EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "VisualEditor0",
            "Effect": "Allow",
            "Action": [
                "iam:CreateUser",
                "iam:CreateAccessKey"
            ],
            "Resource": "*"
        },
        {
            "Sid": "VisualEditor1",
            "Effect": "Allow",
            "Action": "eks:DescribeCluster",
            "Resource": "*"
        }
    ]
}
EOF

# create the AWS policy
POLICY_ARN=$(aws iam create-policy --policy-name $LIQO_POLICY_NAME --policy-document file://policy | jq -r '.Policy.Arn')

# bind the policy to the liqo user
aws iam attach-user-policy --policy-arn "$POLICY_ARN" --user-name "$LIQO_USER_NAME"
```

Retrieve information about your clusters, typing:
```bash
POD_CIDR=$(aws eks describe-cluster --name ${LIQO_CLUSTER_NAME} --region ${LIQO_CLUSTER_REGION} | jq -r '.cluster.resourcesVpcConfig.vpcId' | xargs aws ec2 describe-vpcs --vpc-ids --region ${LIQO_CLUSTER_REGION} | jq -r '.Vpcs[0].CidrBlock')
```
{{% /expand%}}

{{%expand " Azure Kubernetes Service (AKS)" %}}

AKS clusters have by default with the following PodCIDR and ServiceCIDR:

If you are using Azure CNI:

```bash
SUBNET_ID=$(az aks list --query="[?name=='__YOUR_CLUSTER_NAME__']" | jq -r '.[0].agentPoolProfiles[0].vnetSubnetId')
POD_CIDR=$(az network vnet subnet show --ids ${SUBNET_ID} | jq -r .addressPrefix)
SERVICE_CIDR=$(az aks list --query="[?name=='__YOUR_CLUSTER_NAME__']" | jq -r ".[0].networkProfile.serviceCidr")
```

Or Kubenet:

```bash
POD_CIDR=$(az network vnet subnet show --ids ${SUBNET_ID} | jq -r ".[0].networkProfile.serviceCidr)
SERVICE_CIDR=$(az aks list --query="[?name=='__YOUR_CLUSTER_NAME__']" | jq -r ".[0].networkProfile.serviceCidr")
SERVICE_CIDR=$(az aks list --query="[?name=='__YOUR_CLUSTER_NAME__']" | jq -r ".[0].networkProfile.serviceCidr")
```


{{% /expand%}}
{{%expand " Google Kubernetes Engine (GKE)" %}}

```bash
SERVICE_CIDR=$(gcloud container clusters describe ${LIQO_CLUSTER_NAME} --zone -__YOUR_ZONE__ --project __YOUR_PROJECT_ID__ --format="json" | jq -r '.servicesIpv4Cidr')
POD_CIDR=$(gcloud container clusters describe ${LIQO_CLUSTER_NAME} --zone -__YOUR_ZONE__ --project __YOUR_PROJECT_ID__ --format="json" | jq -r '.clusterIpv4Cidr')
```

{{% /expand%}}
{{%expand "K3s" %}}

K3s clusters have by default with the following PodCIDR and ServiceCIDR:

| Variable               | Default | Description                                 |
| ---------------------- | ------- | ------------------------------------------- |
| `networkManager.config.podCIDR`             |    10.42.0.0/16     |
| `networkManager.config.serviceCIDR`         |    10.43.0.0/16     |
{{% /expand%}}

#### Set-Up Liqo Repository

Firstly, you should add the official Liqo repository to your Helm Configuration:

```bash
helm repo add liqo https://helm.liqo.io/
```

#### Set-up

The most important values you can set are the following:

| Variable               | Description                                 |
| ---------------------- | ------------------------------------------- |
| `networkManager.config.podCIDR`        | The cluster Pod CIDR                                 |
| `networkManager.config.serviceCIDR`    | The cluster Service CIDR                             |
| `discovery.config.clusterLabels`       | Labels used to characterize your cluster's resources |
| `auth.config.allowEmptyToken`          | Enable/disable [cluster pre-authentication](/User/Configure/Authentication)            |

##### Additional AWS values

| Variable               | Description                                 |
| ---------------------- | ------------------------------------------- |
| `awsConfig.accessKeyId`        | The Liqo user AccessKeyId                                 |
| `awsConfig.secretAccessKey`    | The Liqo user SecretAccessKey                             |
| `awsConfig.region`       | The region where your local EKS cluster is deployed |
| `awsConfig.clusterName`          | The name of your local EKS cluster (the one used when you deployed it)            |

Example:

```bash
API_SERVER=$(kubectl config view --minify | grep server | cut -f 2- -d ":" | tr -d " ")
helm install liqo liqo/liqo -n liqo --create-namespace  \
    --set networkManager.config.podCIDR="${POD_CIDR}" \
    --set networkManager.config.serviceCIDR="${SERVICE_CIDR}" \
    --set discovery.config.clusterLabels.region="A" \
    --set discovery.config.clusterLabels.foo="bar" 
```

After a couple of minutes, the installation process will be completed. You can check if Liqo is running by:

```bash
kubectl get pods -n liqo
```

You should see a similar output:

```bash

```

#### Next Steps

After you have successfully installed Liqo, you may:

* [Configure](/user/configure): configure the Liqo security, the automatic discovery of new clusters and other system parameters.
* [Use](/user/use) Liqo: orchestrate your applications across multiple clusters.
