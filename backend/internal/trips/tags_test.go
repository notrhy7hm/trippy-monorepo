package trips

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []string
		want []string
		err  error
	}{
		{
			name: "lowercases ascii input",
			in:   []string{"Driver", "Photographer"},
			want: []string{"driver", "photographer"},
		},
		{
			name: "trims, collapses whitespace, dedupes",
			in:   []string{" driver ", "driver", "food planner"},
			want: []string{"driver", "food-planner"},
		},
		{
			name: "empty input returns empty slice",
			in:   []string{},
			want: []string{},
		},
		{
			name: "nil input returns empty slice",
			in:   nil,
			want: []string{},
		},
		{
			name: "whitespace-only tags are dropped",
			in:   []string{"", "   "},
			want: []string{},
		},
		{
			name: "preserves first-occurrence dedupe order",
			in:   []string{"b", "a", "b", "c", "a"},
			want: []string{"b", "a", "c"},
		},
		{
			name: "duplicates do not trip the eight-tag cap",
			in: []string{
				"driver", "driver", "driver", "driver",
				"driver", "driver", "driver", "driver", "driver",
			},
			want: []string{"driver"},
		},
		{
			name: "exactly eight distinct tags is allowed",
			in:   []string{"a", "b", "c", "d", "e", "f", "g", "h"},
			want: []string{"a", "b", "c", "d", "e", "f", "g", "h"},
		},
		{
			name: "more than eight distinct tags rejected",
			in:   []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
			err:  ErrTooManyTags,
		},
		{
			name: "twenty-four char tag is allowed",
			in:   []string{strings.Repeat("a", 24)},
			want: []string{strings.Repeat("a", 24)},
		},
		{
			name: "twenty-five char tag rejected",
			in:   []string{strings.Repeat("a", 25)},
			err:  ErrTagTooLong,
		},
		{
			name: "invalid character slash rejected",
			in:   []string{"bad/tag"},
			err:  ErrInvalidTag,
		},
		{
			name: "invalid character exclamation rejected",
			in:   []string{"oops!"},
			err:  ErrInvalidTag,
		},
		{
			name: "non-ascii input rejected (tags are ASCII-only)",
			in:   []string{"фото"},
			err:  ErrInvalidTag,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeTags(tc.in)
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Fatalf("err = %v, want %v", err, tc.err)
				}
				if got != nil {
					t.Fatalf("error case must return nil slice, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
