package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/yaml"
	kubernetes "k8s.io/client-go/kubernetes/scheme"
)

const maximumManifest = 4 << 20

func labelsValid(labels map[string]string) bool {
	for key, value := range labels {
		if len(validation.IsQualifiedName(key)) != 0 || len(validation.IsValidLabelValue(value)) != 0 {
			return false
		}
	}
	return true
}

func selects(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for key, value := range selector {
		actual, exists := labels[key]
		if !exists || actual != value {
			return false
		}
	}
	return true
}

func verify(data []byte, effectiveNamespace string) (map[string]any, error) {
	namespace := func(value string) string {
		if value == "" {
			return effectiveNamespace
		}
		return value
	}
	decoder := serializer.NewCodecFactory(kubernetes.Scheme, serializer.EnableStrict).UniversalDeserializer()
	reader := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	objects := make(map[string]runtime.Object)
	counts := make(map[string]int)
	for index := 0; ; index++ {
		var raw json.RawMessage
		if err := reader.Decode(&raw); err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.New("yaml_decode_rejected")
		}
		if index >= 256 {
			return nil, errors.New("document_limit")
		}
		if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		object, kind, err := decoder.Decode(raw, nil, nil)
		if err != nil {
			return nil, errors.New("official_typed_decode_rejected")
		}
		accessor, err := meta.Accessor(object)
		if err != nil || accessor.GetName() == "" || !labelsValid(accessor.GetLabels()) {
			return nil, errors.New("metadata_rejected")
		}
		problems := validation.IsDNS1123Subdomain(accessor.GetName())
		if kind.Kind == "Service" {
			problems = validation.IsDNS1035Label(accessor.GetName())
		}
		if len(problems) != 0 {
			return nil, errors.New("object_name_rejected")
		}
		scope := namespace(accessor.GetNamespace())
		if kind.Kind == "ClusterRole" || kind.Kind == "ClusterRoleBinding" {
			if accessor.GetNamespace() != "" {
				return nil, errors.New("cluster_namespace_rejected")
			}
			scope = ""
		} else if len(validation.IsDNS1123Label(scope)) != 0 {
			return nil, errors.New("object_namespace_rejected")
		}
		key := kind.Kind + "/" + scope + "/" + accessor.GetName()
		if _, exists := objects[key]; exists {
			return nil, errors.New("duplicate_object")
		}
		objects[key] = object
		counts[kind.Kind]++
	}
	if counts["Deployment"] != 1 || counts["Service"] != 1 || counts["ServiceAccount"] != 1 || counts["PersistentVolumeClaim"] != 1 {
		return nil, errors.New("required_object_inventory")
	}
	var deployment *appsv1.Deployment
	for _, object := range objects {
		if item, ok := object.(*appsv1.Deployment); ok {
			deployment = item
		}
	}
	if deployment == nil || deployment.Spec.Selector == nil || !labelsValid(deployment.Spec.Selector.MatchLabels) || !labelsValid(deployment.Spec.Template.Labels) || !selects(deployment.Spec.Selector.MatchLabels, deployment.Spec.Template.Labels) {
		return nil, errors.New("deployment_selector_rejected")
	}
	deploymentNamespace := namespace(deployment.Namespace)
	if _, exists := objects["ServiceAccount/"+deploymentNamespace+"/"+deployment.Spec.Template.Spec.ServiceAccountName]; !exists {
		return nil, errors.New("service_account_reference_rejected")
	}
	claims := 0
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		if _, exists := objects["PersistentVolumeClaim/"+deploymentNamespace+"/"+volume.PersistentVolumeClaim.ClaimName]; !exists {
			return nil, errors.New("claim_reference_rejected")
		}
		claims++
	}
	if claims != 1 {
		return nil, errors.New("runtime_claim_inventory")
	}
	var service *corev1.Service
	for _, object := range objects {
		switch item := object.(type) {
		case *corev1.Service:
			if namespace(item.Namespace) != deploymentNamespace || item.Name != deployment.Name || !labelsValid(item.Spec.Selector) || !selects(item.Spec.Selector, deployment.Spec.Template.Labels) {
				return nil, errors.New("service_selector_rejected")
			}
			service = item
		case *corev1.ServiceAccount, *corev1.PersistentVolumeClaim, *appsv1.Deployment, *networkingv1.Ingress, *rbacv1.Role, *rbacv1.ClusterRole, *rbacv1.RoleBinding, *rbacv1.ClusterRoleBinding:
		default:
			return nil, errors.New("unexpected_object_kind")
		}
	}
	if service == nil {
		return nil, errors.New("service_missing")
	}
	for _, object := range objects {
		switch item := object.(type) {
		case *networkingv1.Ingress:
			if namespace(item.Namespace) != deploymentNamespace {
				return nil, errors.New("ingress_namespace_rejected")
			}
			for _, rule := range item.Spec.Rules {
				if rule.HTTP == nil {
					return nil, errors.New("ingress_http_missing")
				}
				for _, path := range rule.HTTP.Paths {
					if path.Backend.Service == nil || path.Backend.Service.Name != service.Name || path.Backend.Service.Port.Name != "http" {
						return nil, errors.New("ingress_service_reference_rejected")
					}
				}
			}
		case *rbacv1.RoleBinding:
			if item.RoleRef.APIGroup != rbacv1.GroupName || item.RoleRef.Kind != "Role" {
				return nil, errors.New("role_reference_rejected")
			}
			if _, exists := objects["Role/"+namespace(item.Namespace)+"/"+item.RoleRef.Name]; !exists {
				return nil, errors.New("role_missing")
			}
			if err := subjectsMatch(item.Subjects, deployment.Spec.Template.Spec.ServiceAccountName, deploymentNamespace); err != nil {
				return nil, err
			}
		case *rbacv1.ClusterRoleBinding:
			if item.RoleRef.APIGroup != rbacv1.GroupName || item.RoleRef.Kind != "ClusterRole" {
				return nil, errors.New("cluster_role_reference_rejected")
			}
			if _, exists := objects["ClusterRole//"+item.RoleRef.Name]; !exists {
				return nil, errors.New("cluster_role_missing")
			}
			if err := subjectsMatch(item.Subjects, deployment.Spec.Template.Spec.ServiceAccountName, deploymentNamespace); err != nil {
				return nil, err
			}
		}
	}
	return map[string]any{"status": "passed", "objects": len(objects), "kinds": counts, "typed_decode": true, "metadata_and_selectors": true, "service_account_claim_ingress_rbac_references": true, "scope": "local typed YAML and reference validation only; no API server or installation proof"}, nil
}

func subjectsMatch(subjects []rbacv1.Subject, account, scope string) error {
	if len(subjects) != 1 || subjects[0].Kind != "ServiceAccount" || subjects[0].Name != account || subjects[0].Namespace != scope {
		return errors.New("role_subject_rejected")
	}
	return nil
}

func main() {
	effectiveNamespace := "default"
	if len(os.Args) == 3 && os.Args[1] == "--namespace" {
		effectiveNamespace = os.Args[2]
	} else if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "namespace_argument_rejected")
		os.Exit(1)
	}
	if len(effectiveNamespace) < 1 || len(effectiveNamespace) > 63 || len(validation.IsDNS1123Label(effectiveNamespace)) != 0 {
		fmt.Fprintln(os.Stderr, "namespace_argument_rejected")
		os.Exit(1)
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, maximumManifest+1))
	if err != nil || len(data) == 0 || len(data) > maximumManifest {
		fmt.Fprintln(os.Stderr, "manifest_bound_rejected")
		os.Exit(1)
	}
	result, err := verify(data, effectiveNamespace)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		os.Exit(1)
	}
}
