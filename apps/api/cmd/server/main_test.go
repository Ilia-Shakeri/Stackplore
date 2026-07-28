package main

import "testing"

func TestListenerAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{
			name: "default",
			want: defaultHTTPAddress,
		},
		{
			name:  "IPv4 loopback",
			value: "127.0.0.1:9000",
			want:  "127.0.0.1:9000",
		},
		{
			name:  "IPv6 loopback",
			value: "[::1]:9000",
			want:  "[::1]:9000",
		},
		{
			name:  "localhost",
			value: "localhost:9000",
			want:  "localhost:9000",
		},
		{
			name:    "wildcard",
			value:   "0.0.0.0:9000",
			wantErr: true,
		},
		{
			name:    "remote address",
			value:   "192.0.2.1:9000",
			wantErr: true,
		},
		{
			name:    "missing host",
			value:   ":9000",
			wantErr: true,
		},
		{
			name:    "missing port",
			value:   "127.0.0.1",
			wantErr: true,
		},
		{
			name:    "zero port",
			value:   "127.0.0.1:0",
			wantErr: true,
		},
		{
			name:    "named port",
			value:   "127.0.0.1:http",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := listenerAddress(func(string) string {
				return test.value
			})

			if test.wantErr {
				if err == nil {
					t.Fatal("listenerAddress() error = nil; want error")
				}
				return
			}

			if err != nil {
				t.Fatalf("listenerAddress() error = %v", err)
			}

			if got != test.want {
				t.Errorf("listenerAddress() = %q; want %q", got, test.want)
			}
		})
	}
}
