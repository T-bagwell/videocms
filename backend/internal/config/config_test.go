package config

import (
	"reflect"
	"testing"
)

func TestEnvList(t *testing.T) {
	t.Setenv("TEST_CORS_LIST", " https://a.example , b.example ,, https://c.example ")
	got := envList("TEST_CORS_LIST")
	want := []string{"https://a.example", "b.example", "https://c.example"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("envList = %v, want %v", got, want)
	}

	t.Setenv("TEST_CORS_LIST", "")
	if got := envList("TEST_CORS_LIST"); got != nil {
		t.Fatalf("empty envList = %v, want nil", got)
	}
}
