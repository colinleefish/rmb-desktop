package appshell

import (
	"testing"
)

func TestNormalizeAddr(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "http://" + DefaultAddr},
		{"   ", "http://" + DefaultAddr},
		{"127.0.0.1:19019", "http://127.0.0.1:19019"},
		{"127.0.0.1:19019/", "http://127.0.0.1:19019"},
		{"http://127.0.0.1:19019", "http://127.0.0.1:19019"},
		{"http://127.0.0.1:19019/", "http://127.0.0.1:19019"},
		{"https://example.com/", "https://example.com"},
		{"  127.0.0.1:8080  ", "http://127.0.0.1:8080"},
	}
	for _, tc := range cases {
		if got := NormalizeAddr(tc.in); got != tc.want {
			t.Errorf("NormalizeAddr(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBaseURLPrecedence(t *testing.T) {
	t.Setenv("RMB_ADDR", "127.0.0.1:19999")
	if got := BaseURL(); got != "http://127.0.0.1:19999" {
		t.Errorf("env addr: got %q", got)
	}

	t.Setenv("RMB_ADDR", "")
	// No config.yaml in the test harness's platform dir → default.
	if got := BaseURL(); got != "http://"+DefaultAddr {
		t.Errorf("default addr: got %q", got)
	}
}

func TestDaemonPort(t *testing.T) {
	cases := []struct {
		addr string
		want uint16
	}{
		{"127.0.0.1:19019", 19019},
		{"http://127.0.0.1:8080", 8080},
		{"https://example.com", 443},
		{"http://example.com", 80},
	}
	for _, tc := range cases {
		t.Setenv("RMB_ADDR", tc.addr)
		if got := DaemonPort(); got != tc.want {
			t.Errorf("DaemonPort(%q) = %d, want %d", tc.addr, got, tc.want)
		}
	}
}

func TestDashboardURL(t *testing.T) {
	t.Setenv("RMB_ADDR", "127.0.0.1:19019/")
	if got := DashboardURL(); got != "http://127.0.0.1:19019/ui/" {
		t.Errorf("DashboardURL = %q", got)
	}
}
