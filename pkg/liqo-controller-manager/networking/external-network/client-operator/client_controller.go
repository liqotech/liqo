// Copyright 2019-2025 The Liqo Authors
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

package clientoperator

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	labelsutils "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/utils"
	dynamicutils "github.com/liqotech/liqo/pkg/utils/dynamic"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// ClientReconciler manage GatewayClient lifecycle.
type ClientReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	DynClient       dynamic.Interface
	Factory         *dynamicutils.RunnableFactory
	ClientResources []string

	eventRecorder record.EventRecorder
}

type templateData struct {
	Spec       networkingv1beta1.GatewayClientSpec
	Name       string
	Namespace  string
	GatewayUID string
	ClusterID  string
	SecretName string
}

// NewClientReconciler returns a new ClientReconciler.
func NewClientReconciler(cl client.Client, dynClient dynamic.Interface,
	factory *dynamicutils.RunnableFactory, s *runtime.Scheme,
	eventRecorder record.EventRecorder,
	clientResources []string) *ClientReconciler {
	return &ClientReconciler{
		Client:          cl,
		Scheme:          s,
		DynClient:       dynClient,
		Factory:         factory,
		ClientResources: clientResources,

		eventRecorder: eventRecorder,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayclients,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayclients/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayclients,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayclients/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayclienttemplates,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage GatewayClient lifecycle.
func (r *ClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	gwClient := &networkingv1beta1.GatewayClient{}
	if err = r.Get(ctx, req.NamespacedName, gwClient); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Gateway client %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the gateway client %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	defer func() {
		newErr := r.Status().Update(ctx, gwClient)
		if newErr != nil {
			if err != nil {
				klog.Errorf("Error reconciling the gateway client %q: %s", req.NamespacedName, err)
			}
			klog.Errorf("Unable to update the gateway client %q: %s", req.NamespacedName, newErr)
			err = newErr
			return
		}

		r.eventRecorder.Eventf(gwClient, corev1.EventTypeNormal, "Reconciled", "Reconciled GatewayClient %q", gwClient.Name)
	}()

	if err = r.EnsureGatewayClient(ctx, gwClient); err != nil {
		klog.Errorf("Unable to ensure the gateway client %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// EnsureGatewayClient ensures the GatewayClient is correctly configured.
func (r *ClientReconciler) EnsureGatewayClient(ctx context.Context, gwClient *networkingv1beta1.GatewayClient) error {
	if gwClient.Labels == nil {
		gwClient.Labels = map[string]string{}
	}
	remoteClusterID, ok := gwClient.Labels[consts.RemoteClusterID]
	if !ok {
		return fmt.Errorf("missing label %q on GatewayClient %q", consts.RemoteClusterID, gwClient.Name)
	}

	templateGV, err := schema.ParseGroupVersion(gwClient.Spec.ClientTemplateRef.APIVersion)
	if err != nil {
		return fmt.Errorf("unable to parse the client template group version: %w", err)
	}

	templateGVR := schema.GroupVersionResource{
		Group:    templateGV.Group,
		Version:  templateGV.Version,
		Resource: enutils.KindToResource(gwClient.Spec.ClientTemplateRef.Kind),
	}
	template, err := r.DynClient.Resource(templateGVR).
		Namespace(gwClient.Spec.ClientTemplateRef.Namespace).
		Get(ctx, gwClient.Spec.ClientTemplateRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get the client template: %w", err)
	}

	templateSpec, ok := template.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the spec of the client template")
	}
	objectKindInt, ok := templateSpec["objectKind"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the object kind of the client template")
	}
	objectKind := metav1.TypeMeta{
		Kind:       objectKindInt["kind"].(string),
		APIVersion: objectKindInt["apiVersion"].(string),
	}
	objectTemplate, ok := templateSpec["template"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the template of the client template")
	}
	objectTemplateMetadata, ok := objectTemplate["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the metadata of the client template")
	}
	objectTemplateSpec, ok := objectTemplate["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the spec of the client template")
	}

	unstructuredObject, err := dynamicutils.CreateOrPatch(ctx, r.DynClient.Resource(objectKind.GroupVersionKind().
		GroupVersion().WithResource(enutils.KindToResource(objectKind.Kind))).
		Namespace(gwClient.Namespace), gwClient.Name, func(objChild *unstructured.Unstructured) error {
		objChild.SetGroupVersionKind(objectKind.GroupVersionKind())

		td := templateData{
			Spec:       gwClient.Spec,
			Name:       gwClient.Name,
			Namespace:  gwClient.Namespace,
			GatewayUID: string(gwClient.UID),
			ClusterID:  remoteClusterID,
			SecretName: gwClient.Spec.SecretRef.Name,
		}

		name, err := enutils.RenderTemplate(objectTemplateMetadata["name"], td, true)
		if err != nil {
			return fmt.Errorf("unable to render the template name: %w", err)
		}
		objChild.SetName(name.(string))

		namespace, err := enutils.RenderTemplate(objectTemplateMetadata["namespace"], td, true)
		if err != nil {
			return fmt.Errorf("unable to render the template namespace: %w", err)
		}
		objChild.SetNamespace(namespace.(string))

		var objChildMetadata map[string]interface{}
		objChildMetadata, ok = objChild.Object["metadata"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("unable to get the child object metadata")
		}

		var objectTemplateMetadataLabels interface{}
		if objectTemplateMetadataLabels, ok = objectTemplateMetadata["labels"]; ok {
			labels, err := enutils.RenderTemplate(objectTemplateMetadataLabels, td, true)
			if err != nil {
				return fmt.Errorf("unable to render the template labels: %w", err)
			}
			objChildMetadata["labels"] = labels
		}

		resource.AddGlobalLabels(objChild)

		var objectTemplateMetadataAnnotations interface{}
		if objectTemplateMetadataAnnotations, ok = objectTemplateMetadata["annotations"]; ok {
			annotations, err := enutils.RenderTemplate(objectTemplateMetadataAnnotations, td, true)
			if err != nil {
				return fmt.Errorf("unable to render the template annotations: %w", err)
			}
			objChildMetadata["annotations"] = annotations
		}

		resource.AddGlobalAnnotations(objChild)

		objChild.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: gwClient.APIVersion,
				Kind:       gwClient.Kind,
				Name:       gwClient.Name,
				UID:        gwClient.UID,
				Controller: ptr.To(true),
			},
		})

		objChild.SetLabels(labelsutils.Merge(objChild.GetLabels(), labelsutils.Set{consts.RemoteClusterID: remoteClusterID}))

		spec, err := enutils.RenderTemplate(objectTemplateSpec, td, false)
		if err != nil {
			return fmt.Errorf("unable to render the template spec: %w", err)
		}
		objChild.Object["spec"] = spec
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to update the client: %w", err)
	}

	gwClient.Status.ClientRef = &corev1.ObjectReference{
		APIVersion: unstructuredObject.GetAPIVersion(),
		Kind:       unstructuredObject.GetKind(),
		Name:       unstructuredObject.GetName(),
		Namespace:  unstructuredObject.GetNamespace(),
		UID:        unstructuredObject.GetUID(),
	}

	status, ok := unstructuredObject.Object["status"].(map[string]interface{})
	if !ok {
		// the object does not have a status
		return nil
	}
	secretRef, ok := enutils.GetIfExists[map[string]interface{}](status, "secretRef")
	if ok && secretRef != nil {
		gwClient.Status.SecretRef = enutils.ParseRef(*secretRef)
	}
	internalEndpoint, ok := enutils.GetIfExists[map[string]interface{}](status, "internalEndpoint")
	if ok && internalEndpoint != nil {
		gwClient.Status.InternalEndpoint = enutils.ParseInternalEndpoint(*internalEndpoint)
	}

	return nil
}

// SetupWithManager register the ClientReconciler to the manager.
func (r *ClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ownerEnqueuer := enutils.NewOwnerEnqueuer(networkingv1beta1.GatewayClientKind)
	factorySource := dynamicutils.NewFactorySource(r.Factory)

	for _, resource := range r.ClientResources {
		gvr, err := enutils.ParseGroupVersionResource(resource)
		if err != nil {
			return err
		}
		factorySource.ForResource(gvr)
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlGatewayClientExternal).
		WatchesRawSource(factorySource.Source(ownerEnqueuer)).
		For(&networkingv1beta1.GatewayClient{}).
		Complete(r)
}
