package source

import (
	"k8s.io/apimachinery/pkg/labels"

	kdsource "github.com/somaz94/kube-diff/pkg/source"
)

// NewTargetFilter narrows src to resources in namespaces (when non-empty) whose
// labels match selector (when non-nil). It returns src unchanged when neither is
// set. A resource without a namespace never matches a namespace filter, since
// its effective namespace is unknown.
func NewTargetFilter(src kdsource.Source, namespaces []string, selector labels.Selector) kdsource.Source {
	if len(namespaces) == 0 && selector == nil {
		return src
	}
	f := &targetFilter{src: src, selector: selector}
	if len(namespaces) > 0 {
		f.namespaces = make(map[string]bool, len(namespaces))
		for _, ns := range namespaces {
			f.namespaces[ns] = true
		}
	}
	return f
}

type targetFilter struct {
	src        kdsource.Source
	namespaces map[string]bool
	selector   labels.Selector
}

func (f *targetFilter) Load() ([]kdsource.Resource, error) {
	all, err := f.src.Load()
	if err != nil {
		return nil, err
	}
	kept := make([]kdsource.Resource, 0, len(all))
	for _, r := range all {
		if f.namespaces != nil && !f.namespaces[r.Namespace] {
			continue
		}
		if f.selector != nil {
			var lbls map[string]string
			if r.Object != nil {
				lbls = r.Object.GetLabels()
			}
			if !f.selector.Matches(labels.Set(lbls)) {
				continue
			}
		}
		kept = append(kept, r)
	}
	return kept, nil
}
