// Copyright 2019-2023 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package forge

import "fmt"

const (
	// EventSuccessfulReflection -> the reason for the event when the reflection completes successfully.
	EventSuccessfulReflection = "SuccessfulReflection"

	// EventFailedReflection -> the reason for the event when the reflection fails.
	EventFailedReflection = "FailedReflection"

	// EventFailedDeletion -> the reason for the event when the deletion of an object fails.
	EventFailedDeletion = "FailedDeletion"

	// EventReflectionDisabled -> the reason for the event when reflection is disabled for the given namespace/object.
	EventReflectionDisabled = "ReflectionDisabled"

	// EventSuccessfulSATokensReflection -> the reason for the event when the reflection of service account tokens completes successfully.
	EventSuccessfulSATokensReflection = "SuccessfulSATokensReflection"

	// EventFailedSATokensReflection -> the reason for the event when the reflection of service account tokens fails.
	EventFailedSATokensReflection = "FailedSATokensReflection"
)

// EventSuccessfulReflectionMsg returns the message for the event when the outgoing reflection completes successfully.
func EventSuccessfulReflectionMsg() string {
	return fmt.Sprintf("Successfully reflected object to cluster %q", RemoteCluster.ClusterName)
}

// EventSuccessfulStatusReflectionMsg returns the message for the event when the incoming reflection completes successfully.
func EventSuccessfulStatusReflectionMsg() string {
	return fmt.Sprintf("Successfully reflected object status back from cluster %q", RemoteCluster.ClusterName)
}

// EventFailedReflectionMsg returns the message for the event when the outgoing reflection fails due to an error.
func EventFailedReflectionMsg(err error) string {
	return fmt.Sprintf("Error reflecting object to cluster %q: %v", RemoteCluster.ClusterName, err)
}

// EventFailedStatusReflectionMsg returns the message for the event when the incoming reflection fails due to an error.
func EventFailedStatusReflectionMsg(err error) string {
	return fmt.Sprintf("Error reflecting object status back from cluster %q: %v", RemoteCluster.ClusterName, err)
}

// EventFailedReflectionAlreadyExistsMsg returns the message for the event when the reflection
// has been aborted because the remote object already exists.
func EventFailedReflectionAlreadyExistsMsg() string {
	return fmt.Sprintf("Error reflecting object to cluster %q: remote object already exists", RemoteCluster.ClusterName)
}

// EventFailedLabelsUpdateMsg returns the message for the event when it is impossible to update the labels of a local object.
func EventFailedLabelsUpdateMsg(err error) string {
	return fmt.Sprintf("Error updating local object labels: %v", err)
}

// EventFailedDeletionMsg returns the message for the event when the deletion of a local object fails.
func EventFailedDeletionMsg(err error) string {
	return fmt.Sprintf("Error deleting local object: %v", err)
}

// EventReflectionDisabledMsg returns the message for the event when reflection is disabled for the given namespace.
func EventReflectionDisabledMsg(namespace string) string {
	return fmt.Sprintf("Reflection to cluster %q disabled for namespace %q", RemoteCluster.ClusterName, namespace)
}

// EventReflectionDisabledErrorMsg returns the message for the event when reflection is disabled for the given namespace, and an error occurs.
func EventReflectionDisabledErrorMsg(namespace string, err error) string {
	return fmt.Sprintf("Reflection to cluster %q disabled for namespace %q: error updating status: %v", RemoteCluster.ClusterName, namespace, err)
}

// EventObjectReflectionDisabledMsg returns the message for the event when reflection is disabled for a given resource.
func EventObjectReflectionDisabledMsg() string {
	return fmt.Sprintf("Reflection to cluster %q disabled for the current object", RemoteCluster.ClusterName)
}

// EventSAReflectionDisabledMsg returns the message for the event when service account reflection is disabled.
func EventSAReflectionDisabledMsg() string {
	return fmt.Sprintf("Reflection to cluster %q disabled for secrets holding service account tokens", RemoteCluster.ClusterName)
}
