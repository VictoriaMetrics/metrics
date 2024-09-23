package metrics

import (
	"fmt"
	"testing"
)

// MyLabelsFast will be converted into string
// via custom implementation (Using ToLabelsString() method of LabelComposer interface)
// It's fast but requires manual implementation.
type MyLabelsFast struct {
	AutoLabelComposer
	Status string
	Flag   bool
}

func (m *MyLabelsFast) ToLabelsString() string {
	return "{" +
		`status="` + m.Status + `",` +
		`flag="` + fmt.Sprintf("%t", m.Flag) + `"` +
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

func TestLabelComposeWithReflect(t *testing.T) {
	want := `my_counter{status="active",flag="true"}`

	// MyLabelsSlow will be converted into {hello="world",enabled="true"}
	// via reflect implementation.
	// It's slow but completely automatic. You don't need to write any code
	type MyLabelsAuto struct {
		AutoLabelComposer

		Status string
		Flag   bool
	}

	got := NameComposeAuto("my_counter", MyLabelsAuto{
		Status: "active",
		Flag:   true,
	})

	if got != want {
		t.Fatalf("unexpected full name; got %q; want %q", got, want)
	}
}
