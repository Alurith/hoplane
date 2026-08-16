package connector

import "testing"

func TestExecutableNameValidationIsGenericAndRejectsPaths(t *testing.T) {
	for _, name := range []string{"ssh", "xfreerdp3", "future-rdp-client.exe"} {
		t.Run("valid "+name, func(t *testing.T) {
			if err := validateExecutableName(name); err != nil {
				t.Fatalf("validateExecutableName(%q) error = %v", name, err)
			}
		})
	}

	for _, name := range []string{"", "/usr/bin/ssh", `C:\\Windows\\mstsc.exe`, "../ssh", "rdp client", "client;other"} {
		t.Run("invalid "+name, func(t *testing.T) {
			if err := validateExecutableName(name); err == nil {
				t.Fatalf("validateExecutableName(%q) error = nil, want rejection", name)
			}
		})
	}
}
