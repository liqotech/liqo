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

package serveroperator

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	enutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/utils"
	dynamicutils "github.com/liqotech/liqo/pkg/utils/dynamic"
)

// ServerReconciler manage GatewayServer lifecycle.
type ServerReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	DynClient       dynamic.Interface
	Factory         *dynamicutils.RunnableFactory
	ServerResources []string
}

type templateData struct {
	Spec       networkingv1alpha1.GatewayServerSpec
	Name       string
	Namespace  string
	GatewayUID string
	ClusterID  string
}

// NewServerReconciler returns a new ServerReconciler.
func NewServerReconciler(cl client.Client, dynClient dynamic.Interface,
	factory *dynamicutils.RunnableFactory, s *runtime.Scheme,
	serverResources []string) *ServerReconciler {
	return &ServerReconciler{
		Client:          cl,
		Scheme:          s,
		DynClient:       dynClient,
		Factory:         factory,
		ServerResources: serverResources,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservertemplates,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage GatewayServer lifecycle.
func (r *ServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	server := &networkingv1alpha1.GatewayServer{}
	if err = r.Get(ctx, req.NamespacedName, server); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Gateway server %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the gateway server %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	defer func() {
		newErr := r.Status().Update(ctx, server)
		if newErr != nil {
			if err != nil {
				klog.Errorf("Error reconciling the gateway server %q: %s", req.NamespacedName, err)
			}
			klog.Errorf("Unable to update the gateway server %q: %s", req.NamespacedName, newErr)
			err = newErr
		}
	}()

	if err = r.EnsureGatewayServer(ctx, server); err != nil {
		klog.Errorf("Unable to ensure the gateway server %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// EnsureGatewayServer ensures the GatewayServer is correctly configured.
func (r *ServerReconciler) EnsureGatewayServer(ctx context.Context, server *networkingv1alpha1.GatewayServer) error {
	if server.Labels == nil {
		server.Labels = map[string]string{}
	}
	remoteClusterID, ok := server.Labels[consts.RemoteClusterID]
	if !ok {
		return fmt.Errorf("missing label %q on GatewayServer %q", consts.RemoteClusterID, server.Name)
	}

	templateGV, err := schema.ParseGroupVersion(server.Spec.ServerTemplateRef.APIVersion)
	if err != nil {
		return fmt.Errorf("unable to parse the server template group version: %w", err)
	}

	templateGVR := schema.GroupVersionResource{
		Group:    templateGV.Group,
		Version:  templateGV.Version,
		Resource: enutils.KindToResource(server.Spec.ServerTemplateRef.Kind),
	}
	template, err := r.DynClient.Resource(templateGVR).
		Namespace(server.Spec.ServerTemplateRef.Namespace).
		Get(ctx, server.Spec.ServerTemplateRef.Name, metav1.GetOptions{})
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
	objectTemplateMetadataInt, ok := objectTemplate["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the metadata of the server template")
	}
	objectTemplateMetadata := metav1.ObjectMeta{
		Name:        enutils.GetValueOrDefault(objectTemplateMetadataInt, "name", server.Name),
		Namespace:   enutils.GetValueOrDefault(objectTemplateMetadataInt, "namespace", server.Namespace),
		Labels:      enutils.TranslateMap(objectTemplateMetadataInt["labels"]),
		Annotations: enutils.TranslateMap(objectTemplateMetadataInt["annotations"]),
	}
	objectTemplateSpec, ok := objectTemplate["spec"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("unable to get the spec of the server template")
	}

	unstructuredObject, err := dynamicutils.CreateOrPatch(ctx, r.DynClient.Resource(objectKind.GroupVersionKind().
		GroupVersion().WithResource(enutils.KindToResource(objectKind.Kind))).
		Namespace(server.Namespace), server.Name, func(obj *unstructured.Unstructured) error {
		obj.SetGroupVersionKind(objectKind.GroupVersionKind())
		obj.SetName(server.Name)
		obj.SetNamespace(server.Namespace)
		obj.SetLabels(labels.Merge(objectTemplateMetadata.Labels, labels.Set{consts.RemoteClusterID: remoteClusterID}))
		obj.SetAnnotations(objectTemplateMetadata.Annotations)
		obj.SetOwnerReferences([]metav1.OwnerReference{
			{
				APIVersion: server.APIVersion,
				Kind:       server.Kind,
				Name:       server.Name,
				UID:        server.UID,
				Controller: pointer.Bool(true),
			},
		})
		spec, err := enutils.RenderTemplate(objectTemplateSpec, templateData{
			Spec:       server.Spec,
			Name:       server.Name,
			Namespace:  server.Namespace,
			GatewayUID: string(server.UID),
			ClusterID:  remoteClusterID,
		})
		if err != nil {
			return fmt.Errorf("unable to render the template: %w", err)
		}
		obj.Object["spec"] = spec
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to update the server: %w", err)
	}

	server.Status.ServerRef = &corev1.ObjectReference{
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
		server.Status.Endpoint = enutils.ParseEndpoint(*endpoint)
	}
	secretRef, ok := enutils.GetIfExists[map[string]interface{}](status, "secretRef")
	if ok && secretRef != nil {
		server.Status.SecretRef = enutils.ParseRef(*secretRef)
	}

	return nil
}

// SetupWithManager register the ServerReconciler to the manager.
func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ownerEnqueuer := enutils.NewOwnerEnqueuer(networkingv1alpha1.GatewayServerKind)
	factorySource := dynamicutils.NewFactorySource(r.Factory)

	for _, resource := range r.ServerResources {
		gvr, err := enutils.ParseGroupVersionResource(resource)
		if err != nil {
			return err
		}
		factorySource.ForResource(gvr)
	}

	return ctrl.NewControllerManagedBy(mgr).
		WatchesRawSource(factorySource.Source(), ownerEnqueuer).
		For(&networkingv1alpha1.GatewayServer{}).
		Complete(r)
}
