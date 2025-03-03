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

package serveroperator

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

// ServerReconciler manage GatewayServer lifecycle.
type ServerReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	DynClient       dynamic.Interface
	Factory         *dynamicutils.RunnableFactory
	ServerResources []string

	eventRecorder record.EventRecorder
}

type templateData struct {
	Spec       networkingv1beta1.GatewayServerSpec
	Name       string
	Namespace  string
	GatewayUID string
	ClusterID  string
	SecretName string
}

// NewServerReconciler returns a new ServerReconciler.
func NewServerReconciler(cl client.Client, dynClient dynamic.Interface,
	factory *dynamicutils.RunnableFactory, s *runtime.Scheme,
	eventRecorder record.EventRecorder,
	serverResources []string) *ServerReconciler {
	return &ServerReconciler{
		Client:          cl,
		Scheme:          s,
		DynClient:       dynClient,
		Factory:         factory,
		ServerResources: serverResources,

		eventRecorder: eventRecorder,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservertemplates,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage GatewayServer lifecycle.
func (r *ServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	gwServer := &networkingv1beta1.GatewayServer{}
	if err = r.Get(ctx, req.NamespacedName, gwServer); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Gateway server %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the gateway server %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	defer func() {
		newErr := r.Status().Update(ctx, gwServer)
		if newErr != nil {
			if err != nil {
				klog.Errorf("Error reconciling the gateway server %q: %s", req.NamespacedName, err)
			}
			klog.Errorf("Unable to update the gateway server %q: %s", req.NamespacedName, newErr)
			err = newErr
			return
		}

		r.eventRecorder.Eventf(gwServer, corev1.EventTypeNormal, "Reconciled", "Reconciled GatewayServer %q", gwServer.Name)
	}()

	if err = r.EnsureGatewayServer(ctx, gwServer); err != nil {
		klog.Errorf("Unable to ensure the gateway server %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// EnsureGatewayServer ensures the GatewayServer is correctly configured.
func (r *ServerReconciler) EnsureGatewayServer(ctx context.Context, gwServer *networkingv1beta1.GatewayServer) error {
	if gwServer.Labels == nil {
		gwServer.Labels = map[string]string{}
	}
	remoteClusterID, ok := gwServer.Labels[consts.RemoteClusterID]
	if !ok {
		return fmt.Errorf("missing label %q on GatewayServer %q", consts.RemoteClusterID, gwServer.Name)
	}

	templateGV, err := schema.ParseGroupVersion(gwServer.Spec.ServerTemplateRef.APIVersion)
	if err != nil {
		return fmt.Errorf("unable to parse the server template group version: %w", err)
	}

	templateGVR := schema.GroupVersionResource{
		Group:    templateGV.Group,
		Version:  templateGV.Version,
		Resource: enutils.KindToResource(gwServer.Spec.ServerTemplateRef.Kind),
	}
	template, err := r.DynClient.Resource(templateGVR).
		Namespace(gwServer.Spec.ServerTemplateRef.Namespace).
		Get(ctx, gwServer.Spec.ServerTemplateRef.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("unable to get the server template: %w", err)
	}

	templateSpec, ok := template.Object["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the spec of the server template")
	}
	objectKindInt, ok := templateSpec["objectKind"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the object kind of the server template")
	}
	objectKind := metav1.TypeMeta{
		Kind:       objectKindInt["kind"].(string),
		APIVersion: objectKindInt["apiVersion"].(string),
	}
	objectTemplate, ok := templateSpec["template"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the template of the server template")
	}
	objectTemplateMetadata, ok := objectTemplate["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the metadata of the server template")
	}
	objectTemplateSpec, ok := objectTemplate["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the spec of the server template")
	}

	unstructuredObject, err := dynamicutils.CreateOrPatch(ctx, r.DynClient.Resource(objectKind.GroupVersionKind().
		GroupVersion().WithResource(enutils.KindToResource(objectKind.Kind))).
		Namespace(gwServer.Namespace), gwServer.Name, func(objChild *unstructured.Unstructured) error {
		objChild.SetGroupVersionKind(objectKind.GroupVersionKind())

		td := templateData{
			Spec:       gwServer.Spec,
			Name:       gwServer.Name,
			Namespace:  gwServer.Namespace,
			GatewayUID: string(gwServer.UID),
			ClusterID:  remoteClusterID,
			SecretName: gwServer.Spec.SecretRef.Name,
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
				APIVersion: gwServer.APIVersion,
				Kind:       gwServer.Kind,
				Name:       gwServer.Name,
				UID:        gwServer.UID,
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
		return fmt.Errorf("unable to update the server: %w", err)
	}

	gwServer.Status.ServerRef = &corev1.ObjectReference{
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
	endpoint, ok := enutils.GetIfExists[map[string]interface{}](status, "endpoint")
	if ok && endpoint != nil {
		gwServer.Status.Endpoint = enutils.ParseEndpoint(*endpoint)
	}
	secretRef, ok := enutils.GetIfExists[map[string]interface{}](status, "secretRef")
	if ok && secretRef != nil {
		gwServer.Status.SecretRef = enutils.ParseRef(*secretRef)
	}
	internalEndpoint, ok := enutils.GetIfExists[map[string]interface{}](status, "internalEndpoint")
	if ok && internalEndpoint != nil {
		gwServer.Status.InternalEndpoint = enutils.ParseInternalEndpoint(*internalEndpoint)
	}

	return nil
}

// SetupWithManager register the ServerReconciler to the manager.
func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ownerEnqueuer := enutils.NewOwnerEnqueuer(networkingv1beta1.GatewayServerKind)
	factorySource := dynamicutils.NewFactorySource(r.Factory)

	for _, resource := range r.ServerResources {
		gvr, err := enutils.ParseGroupVersionResource(resource)
		if err != nil {
			return err
		}
		factorySource.ForResource(gvr)
	}

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlGatewayServerExternal).
		WatchesRawSource(factorySource.Source(ownerEnqueuer)).
		For(&networkingv1beta1.GatewayServer{}).
		Complete(r)
}
