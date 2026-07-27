package utils

import (
	"net"
	"reflect"
	"testing"
)

func TestFirstAndLastAvailable(t *testing.T) {
	type args struct {
		network string
	}
	tests := []struct {
		name         string
		args         args
		first        net.IP
		last         net.IP
		failExpected bool
	}{
		{
			name:  "Range .0/24",
			args:  args{network: "10.0.135.0/24"},
			first: net.ParseIP("10.0.135.1"),
			last:  net.ParseIP("10.0.135.254"),
		}, {
			name:  "Range .128/24",
			args:  args{network: "10.0.135.128/24"},
			first: net.ParseIP("10.0.135.1"),
			last:  net.ParseIP("10.0.135.254"),
		}, {
			name:  "Range .135/24",
			args:  args{network: "10.0.135.135/24"},
			first: net.ParseIP("10.0.135.1"),
			last:  net.ParseIP("10.0.135.254"),
		}, {
			name:  "Range .0/25",
			args:  args{network: "192.168.10.0/25"},
			first: net.ParseIP("192.168.10.1"),
			last:  net.ParseIP("192.168.10.126"),
		}, {
			name:  "Range .128/25",
			args:  args{network: "192.168.10.128/25"},
			first: net.ParseIP("192.168.10.129"),
			last:  net.ParseIP("192.168.10.254"),
		}, {
			name:  "Range /30",
			args:  args{network: "192.168.0.0/30"},
			first: net.ParseIP("192.168.0.1"),
			last:  net.ParseIP("192.168.0.2"),
		}, {
			name:         "Range /31",
			args:         args{network: "192.168.0.0/31"},
			first:        nil,
			last:         nil,
			failExpected: true,
		}, {
			name:         "Range /32",
			args:         args{network: "192.168.0.0/32"},
			first:        nil,
			last:         nil,
			failExpected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.failExpected && (tt.first == nil || tt.last == nil) {
				t.Errorf("FirstAndLastAvailable() Unable to parse args")
			}

			_, cidr, _ := net.ParseCIDR(tt.args.network)
			first, last := FirstAndLastAvailable(cidr)
			if !reflect.DeepEqual(first, tt.first) {
				t.Errorf("FirstAndLastAvailable() first = %v, want %v", first, tt.first)
			}
			if !reflect.DeepEqual(last, tt.last) {
				t.Errorf("FirstAndLastAvailable() last = %v, want %v", last, tt.last)
			}
		})
	}
}

func TestAddressRange(t *testing.T) {
	type args struct {
		network string
	}
	tests := []struct {
		name  string
		args  args
		first net.IP
		last  net.IP
	}{
		{
			name:  "Range .0/24",
			args:  args{network: "10.0.135.0/24"},
			first: net.ParseIP("10.0.135.0"),
			last:  net.ParseIP("10.0.135.255"),
		}, {
			name:  "Range .128/24",
			args:  args{network: "10.0.135.128/24"},
			first: net.ParseIP("10.0.135.0"),
			last:  net.ParseIP("10.0.135.255"),
		}, {
			name:  "Range .135/24",
			args:  args{network: "10.0.135.135/24"},
			first: net.ParseIP("10.0.135.0"),
			last:  net.ParseIP("10.0.135.255"),
		}, {
			name:  "Range .0/25",
			args:  args{network: "192.168.10.0/25"},
			first: net.ParseIP("192.168.10.0"),
			last:  net.ParseIP("192.168.10.127"),
		}, {
			name:  "Range .128/25",
			args:  args{network: "192.168.10.128/25"},
			first: net.ParseIP("192.168.10.128"),
			last:  net.ParseIP("192.168.10.255"),
		}, {
			name:  "Range /30",
			args:  args{network: "192.168.0.0/30"},
			first: net.ParseIP("192.168.0.0"),
			last:  net.ParseIP("192.168.0.3"),
		}, {
			name:  "Range /31",
			args:  args{network: "192.168.0.0/31"},
			first: net.ParseIP("192.168.0.0"),
			last:  net.ParseIP("192.168.0.1"),
		}, {
			name:  "Range /32",
			args:  args{network: "192.168.0.0/32"},
			first: net.ParseIP("192.168.0.0"),
			last:  net.ParseIP("192.168.0.0"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cidr, _ := net.ParseCIDR(tt.args.network)
			first, last := AddressRange(cidr)
			if !reflect.DeepEqual(first.String(), tt.first.String()) {
				t.Errorf("AddressRange() first = %v, want %v", first, tt.first)
			}
			if !reflect.DeepEqual(last.String(), tt.last.String()) {
				t.Errorf("AddressRange() last = %v, want %v", last, tt.last)
			}
		})
	}
}

func TestContainsNet(t *testing.T) {
	type args struct {
		net1 string
		net2 string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "", args: args{net1: "192.168.0.0/16", net2: "192.168.0.0/16"}, want: true},
		{name: "", args: args{net1: "192.168.0.0/16", net2: "192.168.0.0/24"}, want: true},
		{name: "", args: args{net1: "192.168.0.0/16", net2: "192.168.0.132/24"}, want: true},
		{name: "", args: args{net1: "192.168.0.0/24", net2: "192.168.0.0/25"}, want: true},
		{name: "", args: args{net1: "192.168.0.0/24", net2: "192.168.0.128/25"}, want: true},
		{name: "", args: args{net1: "192.168.0.0/24", net2: "192.168.0.0/16"}, want: false},
		{name: "", args: args{net1: "192.168.0.0/25", net2: "192.168.0.132/25"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cidr1, _ := net.ParseCIDR(tt.args.net1)
			_, cidr2, _ := net.ParseCIDR(tt.args.net2)

			if got := ContainsNet(cidr1, cidr2); got != tt.want {
				t.Errorf("ContainsNet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsIPInCIDR(t *testing.T) {
	type args struct {
		cidr string
		ip   string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "IPv4 in range",
			args: args{cidr: "192.168.1.0/24", ip: "192.168.1.5"},
			want: true,
		},
		{
			name: "IPv4 at the start of range",
			args: args{cidr: "192.168.1.0/24", ip: "192.168.1.0"},
			want: true,
		},
		{
			name: "IPv4 at the end of range",
			args: args{cidr: "192.168.1.0/24", ip: "192.168.1.255"},
			want: true,
		},
		{
			name: "IPv4 out of range",
			args: args{cidr: "192.168.1.0/24", ip: "192.168.2.1"},
			want: false,
		},
		{
			name: "IPv6 in range",
			args: args{cidr: "2001:db8::/32", ip: "2001:db8::1"},
			want: true,
		},
		{
			name: "IPv6 out of range",
			args: args{cidr: "2001:db8::/32", ip: "2001:db9::1"},
			want: false,
		},
		{
			name:    "Invalid CIDR",
			args:    args{cidr: "invalid", ip: "192.168.1.1"},
			wantErr: true,
		},
		{
			name:    "Invalid IP",
			args:    args{cidr: "192.168.1.0/24", ip: "invalid"},
			wantErr: true,
		},
		{
			name: "Dual-stack CIDR, IPv4 in range",
			args: args{cidr: "10.10.0.0/16,fd00:10:10::/64", ip: "10.10.1.5"},
			want: true,
		},
		{
			name: "Dual-stack CIDR, IPv6 in range",
			args: args{cidr: "10.10.0.0/16,fd00:10:10::/64", ip: "fd00:10:10::1"},
			want: true,
		},
		{
			name: "Dual-stack CIDR, IP out of both ranges",
			args: args{cidr: "10.10.0.0/16,fd00:10:10::/64", ip: "192.168.1.1"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := IsIPInCIDR(tt.args.cidr, tt.args.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsIPInCIDR() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("IsIPInCIDR() = %v, want %v", got, tt.want)
			}
		})
	}
}
