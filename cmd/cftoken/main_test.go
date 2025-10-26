package main

import (
	"reflect"
	"testing"
)

func TestNormalizeCIDRList(t *testing.T) {
	t.Parallel()

	got, disabled, err := normalizeCIDRList([]string{" 10.0.0.1/32 ", "2001:db8::/64"})
	if err != nil {
		t.Fatalf("normalizeCIDRList() error = %v", err)
	}
	if disabled {
		t.Fatalf("normalizeCIDRList() disabled = true, want false")
	}
	want := []string{"10.0.0.1/32", "2001:db8::/64"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeCIDRList() = %v, want %v", got, want)
	}
}

func TestNormalizeCIDRListDisableSentinel(t *testing.T) {
	t.Parallel()

	got, disabled, err := normalizeCIDRList([]string{"0.0.0.0/32"})
	if err != nil {
		t.Fatalf("normalizeCIDRList() error = %v", err)
	}
	if !disabled {
		t.Fatalf("normalizeCIDRList() disabled = false, want true")
	}
	if got != nil {
		t.Fatalf("normalizeCIDRList() returned %+v, want nil", got)
	}
}

func TestNormalizeCIDRListInvalid(t *testing.T) {
	t.Parallel()

	if _, _, err := normalizeCIDRList([]string{"not-a-cidr"}); err == nil {
		t.Fatalf("normalizeCIDRList() error = nil, want error")
	}
}
