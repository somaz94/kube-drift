package source

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	kdsource "github.com/somaz94/kube-diff/pkg/source"
)

type staticSource struct {
	res []kdsource.Resource
	err error
}

func (s staticSource) Load() ([]kdsource.Resource, error) { return s.res, s.err }

func res(name, ns string, lbls map[string]string) kdsource.Resource {
	obj := &unstructured.Unstructured{}
	obj.SetName(name)
	obj.SetNamespace(ns)
	obj.SetLabels(lbls)
	return kdsource.Resource{APIVersion: "v1", Kind: kindConfigMap, Name: name, Namespace: ns, Object: obj}
}

func names(t *testing.T, src kdsource.Source) []string {
	t.Helper()
	got, err := src.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	out := make([]string, 0, len(got))
	for _, r := range got {
		out = append(out, r.Name)
	}
	sort.Strings(out)
	return out
}

func TestNewTargetFilter(t *testing.T) {
	src := staticSource{res: []kdsource.Resource{
		res("a", "team-a", map[string]string{"tier": "web"}),
		res("b", "team-b", map[string]string{"tier": "db"}),
		res("c", "team-a", nil),
		res("cluster", "", map[string]string{"tier": "web"}),
		{APIVersion: "v1", Kind: kindConfigMap, Name: "nil-object", Namespace: "team-a"},
	}}
	web := labels.SelectorFromSet(labels.Set{"tier": "web"})

	tests := []struct {
		name       string
		namespaces []string
		selector   labels.Selector
		want       []string
	}{
		{"namespaces only", []string{"team-a"}, nil, []string{"a", "c", "nil-object"}},
		{"selector only", nil, web, []string{"a", "cluster"}},
		{"both", []string{"team-a", "team-b"}, web, []string{"a"}},
		{"everything selector keeps all", nil, labels.Everything(), []string{"a", "b", "c", "cluster", "nil-object"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := names(t, NewTargetFilter(src, tt.namespaces, tt.selector))
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("kept %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTargetFilter_NoConstraintsReturnsSource(t *testing.T) {
	src := &staticSource{}
	if got := NewTargetFilter(src, nil, nil); got != kdsource.Source(src) {
		t.Errorf("NewTargetFilter without constraints = %T, want the original source", got)
	}
}

func TestNewTargetFilter_PropagatesLoadError(t *testing.T) {
	src := staticSource{err: errors.New("clone failed")}
	if _, err := NewTargetFilter(src, []string{"x"}, nil).Load(); err == nil {
		t.Fatal("expected the source's Load error, got nil")
	}
}
