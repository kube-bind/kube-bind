package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

// IsClaimed returns true if the given object matches the given selector and named resources.
func IsClaimed(selector kubebindv1alpha2.Selector, obj *unstructured.Unstructured) bool {
	// Empty selector selects everything
	if selector.LabelSelector == nil && len(selector.NamedResource) == 0 {
		return true
	}

	// Both label selector and named resources must match if both are specified
	labelSelectorMatches := true
	namedResourceMatches := true

	// Check label selector if specified
	if selector.LabelSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(selector.LabelSelector)
		if err != nil {
			return false
		}
		l := obj.GetLabels()
		if l == nil {
			l = make(map[string]string)
		}
		labelSelectorMatches = selector.Matches(labels.Set(l))
	}

	// Check named resources if specified
	if len(selector.NamedResource) > 0 {
		namedResourceMatches = false // Default to false, must match at least one
		for _, nr := range selector.NamedResource {
			if nr.Namespace != "" && nr.Namespace != obj.GetNamespace() {
				continue
			}
			if nr.Name == obj.GetName() {
				namedResourceMatches = true
				break
			}
		}
	}

	return labelSelectorMatches && namedResourceMatches
}
