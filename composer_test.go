package metrics

import (
	"fmt"
	"testing"
)

// MyLabelsSlow will be converted into {hello="world",enabled="true"}
// via reflect implementation.
// It's slow but completely automatic. You don't need to write any code
type MyLabelsSlow struct {
	StructLabelComposer

	Status string
	Flag   bool
}

func TestLabelComposeWithReflect(t *testing.T) {
	want := `my_counter{status="active",flag="true"}`

	got := NameCompose("my_counter", MyLabelsSlow{
		Status: "active",
		Flag:   true,
	})

	if got != want {
		t.Fatalf("unexpected full name; got %q; want %q", got, want)
	}
}

// MyLabelsFast will be converted into string
// via custom implementation (Using ToLabelsString() method of LabelComposer interface)
// It's fast but requires manual implementation.
type MyLabelsFast struct {
	StructLabelComposer
	Status string
	Flag   bool
}

func (m *MyLabelsFast) ToLabelsString() string {
	return "{" +
		`status="` + m.Status + `",` +
		`flag="` + fmt.Sprintf("%v", m.Flag) + `"` +
		"}"
}

func TestLabelComposeWithoutReflect(t *testing.T) {
	want := `my_counter{status="active",flag="true"}`
	got := NameCompose("my_counter", &MyLabelsFast{
		Status: "active", Flag: true,
	})

	if want != got {
		t.Fatalf("unexpected full name; got %q; want %q", got, want)
	}
}
