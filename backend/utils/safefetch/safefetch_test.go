package safefetch

import "testing"

func TestValidatePublicHTTPURL(t *testing.T) {
	cases := []struct {
		raw    string
		wantOK bool
	}{
		{"https://example.com/a.png", true},
		{"http://example.com/a.png", true},
		{"ftp://example.com/a.png", false},
		{"http://127.0.0.1/a.png", false},
		{"http://localhost/a.png", false},
		{"http://192.168.1.1/a.png", false},
		{"http://10.0.0.1/a.png", false},
		{"http://169.254.169.254/latest/meta-data", false},
		{"", false},
		{"not-a-url", false},
	}
	for _, tc := range cases {
		err := ValidatePublicHTTPURL(tc.raw)
		if tc.wantOK && err != nil {
			t.Fatalf("%q expected ok, got %v", tc.raw, err)
		}
		if !tc.wantOK && err == nil {
			t.Fatalf("%q expected error", tc.raw)
		}
	}
}
