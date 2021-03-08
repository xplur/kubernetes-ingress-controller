package configuration

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	netv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kong/kubernetes-ingress-controller/railgun/controllers"
	"github.com/kong/kubernetes-ingress-controller/railgun/pkg/configsecret"
)

// -----------------------------------------------------------------------------
// Secret Utils - Storage
// -----------------------------------------------------------------------------

// storeIngressObj reconciles storing the YAML contents of Ingress resources (which are managed by Kong)
// from multiple versions which remain supported.
func storeIngressObj(ctx context.Context, c client.Client, log logr.Logger, nsn types.NamespacedName, obj client.Object) (ctrl.Result, error) {
	// TODO need EVENTS here
	// TODO need more status updates

	// if this is an Ingress resource make sure it's managed by Kong
	if obj.GetObjectKind().GroupVersionKind().Kind == "Ingress" {
		if !isManaged(obj.GetAnnotations()) {
			return ctrl.Result{}, nil
		}
	}

	// get the configuration secret namespace
	secretNamespace := os.Getenv(controllers.CtrlNamespaceEnv)
	if secretNamespace == "" {
		return ctrl.Result{}, fmt.Errorf("kong can not be configured because the required %s env var is not present", controllers.CtrlNamespaceEnv)
	}

	// get the configuration secret
	secret, created, err := getOrCreateConfigSecret(ctx, c, secretNamespace)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("kong configuration secret was created elsewhere retrying", "namespace", nsn.Namespace, "ingress", nsn.Name)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	if created {
		log.Info("kong configuration did not exist, was created successfully", "namespace", nsn.Namespace, "ingress", nsn.Name)
		return ctrl.Result{Requeue: true}, nil
	}
	log.Info("kong configuration secret found", "namespace", nsn.Namespace, "name", controllers.ConfigSecretName)

	// The relevant Service referred to by the ingress also needs to be stored in the cache
	switch ing := obj.(type) {
	case *netv1.Ingress:
		for _, rule := range ing.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				// retrieve the Service object for this Ingress record
				svc := new(corev1.Service)
				nsn := types.NamespacedName{Namespace: obj.GetNamespace(), Name: path.Backend.Service.Name}
				if err := c.Get(ctx, nsn, svc); err != nil {
					return ctrl.Result{}, fmt.Errorf("service %s for ingress %s could not be retrieved: %w", nsn.Name, ing.Name, err)
				}

				// store the Service object
				_, err := storeRuntimeObject(ctx, c, secret, svc, nsn)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}
		}
	case *netv1beta1.Ingress:
		for _, rule := range ing.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				// retrieve the Service object for this Ingress record
				svc := new(corev1.Service)
				nsn := types.NamespacedName{Namespace: obj.GetNamespace(), Name: path.Backend.ServiceName}
				if err := c.Get(ctx, nsn, svc); err != nil {
					return ctrl.Result{}, fmt.Errorf("service %s for ingress %s could not be retrieved: %w", nsn.Name, ing.Name, err)
				}

				// store the Service object
				_, err := storeRuntimeObject(ctx, c, secret, svc, nsn)
				if err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{Requeue: true}, nil
			}
		}
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported ingress type %T", ing)
	}

	// before we store configuration data for this Ingress object, ensure that it has our finalizer set
	if !hasFinalizer(obj, KongIngressFinalizer) {
		finalizers := obj.GetFinalizers()
		obj.SetFinalizers(append(finalizers, KongIngressFinalizer))
		if err := c.Update(ctx, obj); err != nil { // TODO: patch here instead of update
			return ctrl.Result{}, err
		}
	}

	// store the ingress record
	requeue, err := storeRuntimeObject(ctx, c, secret, obj, nsn)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeue {
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("kong configuration patched", "namespace", nsn.Namespace, "name", controllers.ConfigSecretName)
	return ctrl.Result{}, nil
}

// storeRuntimeObject stores a runtime.Object in the configuration secret
func storeRuntimeObject(ctx context.Context, c client.Client, secret *corev1.Secret, obj runtime.Object, nsn types.NamespacedName) (requeue bool, err error) {
	// marshal to YAML for storage
	var cfg []byte
	cfg, err = yaml.Marshal(obj)
	if err != nil {
		return false, err
	}

	// patch the secret with the runtime.Object contents
	key := configsecret.KeyFor(obj, nsn)
	secret.Data[key] = cfg
	if err = c.Update(ctx, secret); err != nil { // TODO: patch here instead of update for perf
		if errors.IsConflict(err) {
			requeue = true
			err = nil
			return
		}
		return
	}

	return
}

// cleanupObj ensures that a deleted ingress resource is no longer present in the kong configuration secret.
func cleanupObj(ctx context.Context, c client.Client, log logr.Logger, nsn types.NamespacedName, obj client.Object) (ctrl.Result, error) {
	// TODO need EVENTS here
	// TODO need more status updates

	// get the configuration secret namespace
	secretNamespace := os.Getenv(controllers.CtrlNamespaceEnv)
	if secretNamespace == "" {
		return ctrl.Result{}, fmt.Errorf("kong can not be configured because the required %s env var is not present", controllers.CtrlNamespaceEnv)
	}

	// grab the configuration secret from the API
	secret := new(corev1.Secret)
	if err := c.Get(ctx, types.NamespacedName{Namespace: secretNamespace, Name: controllers.ConfigSecretName}, secret); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	key := configsecret.KeyFor(obj, nsn)
	if _, ok := secret.Data[key]; ok {
		delete(secret.Data, key)
		if err := c.Update(ctx, secret); err != nil { // TODO: patch here instead of update
			return ctrl.Result{}, err
		}
		log.Info("kong ingress record removed from kong configuration", "ingress", obj.GetName(), "config", secret.GetName())
		return ctrl.Result{Requeue: true}, nil
	}

	if hasFinalizer(obj, KongIngressFinalizer) {
		log.Info("kong ingress finalizer needs to be removed from ingress resource which is deleting", "ingress", obj.GetName(), "finalizer", KongIngressFinalizer)
		finalizers := []string{}
		for _, finalizer := range obj.GetFinalizers() {
			if finalizer != KongIngressFinalizer {
				finalizers = append(finalizers, finalizer)
			}
		}
		obj.SetFinalizers(finalizers)
		if err := c.Update(ctx, obj); err != nil { // TODO: patch here instead of update
			return ctrl.Result{}, err
		}
		log.Info("the kong ingress finalizer was removed from an ingress resource which is deleting", "ingress", obj.GetName(), "finalizer", KongIngressFinalizer)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}
