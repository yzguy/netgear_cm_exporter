package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewConfigFromFile(t *testing.T) {
	want := &Config{
		Modem: Modem{
			Address:  "192.168.100.1",
			Username: "admin",
			Password: "foobaz",
			Model:    ModelCM1000,
		},
		Telemetry: Telemetry{
			ListenAddress: ":9527",
			MetricsPath:   "/metrics",
		},
	}

	got, err := NewConfigFromFile("testdata/minimal.yml")
	if err != nil {
		t.Error(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("config differs (-want, +got): %s", diff)
	}

}

func TestNewConfigFromFileModel(t *testing.T) {
	cases := []struct {
		file      string
		wantModel string
	}{
		{file: "testdata/cm3000.yml", wantModel: ModelCM3000},
		{file: "testdata/lowercase_model.yml", wantModel: ModelCM3000},
	}

	for _, c := range cases {
		got, err := NewConfigFromFile(c.file)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.file, err)
			continue
		}
		if got.Modem.Model != c.wantModel {
			t.Errorf("%s: Model = %q, want %q", c.file, got.Modem.Model, c.wantModel)
		}
	}
}

func TestNewConfigFromFileInvalidModel(t *testing.T) {
	if _, err := NewConfigFromFile("testdata/invalid_model.yml"); err == nil {
		t.Error("expected an error for an unsupported modem model, got nil")
	}
}
