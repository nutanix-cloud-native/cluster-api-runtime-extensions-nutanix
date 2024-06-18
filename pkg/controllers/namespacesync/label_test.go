package namespacesync

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespaceHasLabelKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		ns   *corev1.Namespace
		want bool
	}{
		{
			name: "match a labeled namespace",
			key:  "testkey",
			ns: &corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						"testkey": "",
					},
				},
			},
			want: true,
		},
		{
			name: "do not match if label key is not found",
			key:  "testkey",
			ns: &corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
			},
			want: false,
		},
		{
			name: "do not match if label key is empty string",
			key:  "",
			ns: &corev1.Namespace{
				ObjectMeta: v1.ObjectMeta{
					Name: "test",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := NamespaceHasLabelKey(tt.key)
			if got := fn(tt.ns); got != tt.want {
				t.Fatalf("got %t, want %t", got, tt.want)
			}
		})
	}
}
