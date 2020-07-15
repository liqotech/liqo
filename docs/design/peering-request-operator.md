# Peering Request Operator
## Overview
This component manages `PeeringRequest` lifecycle during peering process

### Features
List of supported features
* Accept incoming `PeeringRequest`s
* Starting from accepted `PeeringRequest`s creates related `Broadcaster` Deployment

### Limitations
List of known limitations
* There is no way to define which requests can be accepted and which not, currently accepts all of them
* This component creates a new broadcaster for each `PeeringRequest`, it may be better to have a single broadcaster that manages multiple clusters

## Architecture and workflow

There are two main components:

1. PeeringRequestAdmission
    * validating webhook that take decision about accepting or not incoming `PeeringRequest`
2. PeeringRequestOperator
    * reconciles `PeeringRequest`s checking if a broadcaster for that name already exists, if not it creates it passing PR name as parameter

### Initialization

There are two init containers need by webhook to handle TLS certificate creation:
1. The first one creates certificate and wait for their approval
    * the installer will accept it
    * if you are running without installer make sure to manually accept it
2. The second one creates AdmissionWebhook resource on API Server

### Workflow

The workflow is quite simple:

1. The webhook will accept or refuse incoming `PeeringRequest` from remote cluster
2. The operator reconciles accepted requests and creates Broadcaster Deployment
3. `PeeringRequest` is set as owner of Deployment, so we it will be deleted also broadcaster will be terminated
