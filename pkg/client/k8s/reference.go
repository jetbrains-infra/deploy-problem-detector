package k8s

import (
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	AnnotationDeploymentRevision = "deployment.kubernetes.io/revision"
)

var ErrTooManyKinds = errors.New("too many kinds")

type object interface {
	metav1.Object
	runtime.Object
}

func HasOwnerReference(object metav1.Object, reference metav1.OwnerReference) bool {
	for _, ref := range object.GetOwnerReferences() {
		if ref.UID == reference.UID {
			return true
		}
	}

	return false
}

func NewControllerRef(owner object) (*metav1.OwnerReference, error) {
	gvk, err := getObjectKind(owner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to getTypeInformationOfObject")
	}

	return metav1.NewControllerRef(owner, gvk), nil
}

// getObjectKind gets TypeMeta information of a runtime.Object based upon the loaded scheme.Scheme
func getObjectKind(obj runtime.Object) (schema.GroupVersionKind, error) {
	gvks, _, err := scheme.Scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, errors.Wrap(err, "failed to get ObjectKinds")
	}

	if len(gvks) > 1 {
		return schema.GroupVersionKind{}, errors.Wrapf(ErrTooManyKinds, "expected 1 kind, got: %d: %v", len(gvks), gvks)
	}

	return gvks[0], nil
}

func HasSameRevision(obj1 metav1.Object, obj2 metav1.Object) bool {
	revisionObj1, ok := obj1.GetAnnotations()[AnnotationDeploymentRevision]
	if !ok {
		return false
	}

	revisionObj2, ok := obj2.GetAnnotations()[AnnotationDeploymentRevision]
	if !ok {
		return false
	}

	return revisionObj1 == revisionObj2
}
