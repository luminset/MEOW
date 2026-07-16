package main

import "testing"

func TestIsChinaQQWryLocation(t *testing.T) {
	tests := []struct {
		location string
		direct   bool
		ok       bool
	}{
		{"北京市 电信", true, true},
		{"广东省深圳市 腾讯云", true, true},
		{"局域网 对方和您在同一内部网", true, true},
		{"美国 加利福尼亚州", false, true},
		{"CZ88.NET", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		direct, ok := isChinaQQWryLocation(tt.location)
		if direct != tt.direct || ok != tt.ok {
			t.Errorf("isChinaQQWryLocation(%q) = (%v, %v), want (%v, %v)",
				tt.location, direct, ok, tt.direct, tt.ok)
		}
	}
}
