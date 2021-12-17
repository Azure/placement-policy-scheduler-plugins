package utils

import "testing"

func TestHasMatchingLabels(t *testing.T) {
	test := []struct {
		name          string
		l, wantLabels map[string]string
		want          bool
	}{
		{
			name:       "no labels",
			l:          map[string]string{},
			wantLabels: map[string]string{},
			want:       true,
		},
		{
			name: "actual labels is less than want labels",
			l: map[string]string{
				"foo": "bar",
			},
			wantLabels: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			want: false,
		},
		{
			name: "actual labels don't match want labels",
			l: map[string]string{
				"foo": "bar",
			},
			wantLabels: map[string]string{
				"baz": "qux",
			},
			want: false,
		},
		{
			name: "actual labels match want labels",
			l: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			wantLabels: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			want: true,
		},
		{
			name: "want labels is a subset of actual labels",
			l: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
			wantLabels: map[string]string{
				"foo": "bar",
			},
			want: true,
		},
	}

	for _, tt := range test {
		t.Run(tt.name, func(t *testing.T) {
			got := HasMatchingLabels(tt.l, tt.wantLabels)
			if got != tt.want {
				t.Errorf("HasMatchingLabels(%v, %v) = %v, want %v", tt.l, tt.wantLabels, got, tt.want)
			}
		})
	}
}
