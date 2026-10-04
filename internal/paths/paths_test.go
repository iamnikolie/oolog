package paths

import (
	"path/filepath"
	"testing"
)

func TestHomeRespectsOOLOGHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("OOLOG_HOME", tmp)

	got, err := Home("")
	if err != nil {
		t.Fatal(err)
	}
	if got != tmp {
		t.Fatalf("Home(\"\") = %q, want %q", got, tmp)
	}

	got, err = Home("prod")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(tmp, "prod") {
		t.Fatalf("Home(\"prod\") = %q, want %q", got, filepath.Join(tmp, "prod"))
	}
}
