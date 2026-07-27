package vm

import (
	"strings"
	"testing"

	"github.com/super-phenix/superphenix/internal/superphenix-controller/pkg/config"
)

func setCatalog(t *testing.T, entries []config.ContainerDiskCatalogEntry) {
	t.Helper()
	prev := config.Global.ContainerDiskCatalog
	config.Global.ContainerDiskCatalog = entries
	t.Cleanup(func() { config.Global.ContainerDiskCatalog = prev })
}

var sampleCatalog = []config.ContainerDiskCatalogEntry{
	{ID: "windows-virtio-drivers", DisplayName: "Windows VirtIO Driver", Image: "quay.io/kubevirt/virtio-container-disk:v1.7.0", Bus: "sata", SupportedOS: []string{"windows"}, Recommended: true},
	{ID: "extra-tools", DisplayName: "Extra", Image: "example.com/extra:1", Bus: "virtio", SupportedOS: []string{"ubuntu"}, Recommended: false},
}

func TestSupportsOS(t *testing.T) {
	tests := []struct {
		name         string
		vmPreference string
		supportedOS  []string
		want         bool
	}{
		{name: "exact substring matches", vmPreference: "windows-server-2022", supportedOS: []string{"windows"}, want: true},
		{name: "multiple supportedOS, one matches", vmPreference: "ubuntu-22.04", supportedOS: []string{"windows", "ubuntu"}, want: true},
		{name: "no match", vmPreference: "ubuntu-22.04", supportedOS: []string{"windows"}, want: false},
		{name: "empty preference never matches", vmPreference: "", supportedOS: []string{"windows"}, want: false},
		{name: "empty supportedOS never matches", vmPreference: "windows", supportedOS: []string{}, want: false},
		{name: "empty string in supportedOS does not catch everything", vmPreference: "windows", supportedOS: []string{""}, want: false},
		{name: "nil supportedOS never matches", vmPreference: "windows", supportedOS: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportsOS(tt.vmPreference, tt.supportedOS); got != tt.want {
				t.Fatalf("supportsOS(%q, %v) = %v, want %v", tt.vmPreference, tt.supportedOS, got, tt.want)
			}
		})
	}
}

func TestResolveByIDs(t *testing.T) {
	setCatalog(t, sampleCatalog)

	t.Run("resolves in order with image+bus", func(t *testing.T) {
		got, err := ResolveByIDs([]string{"extra-tools", "windows-virtio-drivers"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got[0].ID != "extra-tools" || got[1].ID != "windows-virtio-drivers" {
			t.Fatalf("unexpected order/result: %+v", got)
		}
		if got[1].Image != "quay.io/kubevirt/virtio-container-disk:v1.7.0" || got[1].Bus != "sata" {
			t.Fatalf("unexpected resolved spec: %+v", got[1])
		}
	})

	t.Run("unknown id rejected with invalid prefix naming the offender", func(t *testing.T) {
		_, err := ResolveByIDs([]string{"nope"})
		if err == nil || !strings.HasPrefix(err.Error(), "invalid container disk") {
			t.Fatalf("want invalid-prefix error, got %v", err)
		}
		if !strings.Contains(err.Error(), "nope") {
			t.Fatalf("error should name the offending id, got %q", err.Error())
		}
	})

	t.Run("existence only - no OS check", func(t *testing.T) {
		// extra-tools supports ubuntu only, but ResolveByIDs ignores OS.
		if _, err := ResolveByIDs([]string{"extra-tools"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("empty input returns empty slice, no error", func(t *testing.T) {
		got, err := ResolveByIDs([]string{})
		if err != nil || len(got) != 0 {
			t.Fatalf("want empty/no-error, got=%+v err=%v", got, err)
		}
	})
}

func TestResolve(t *testing.T) {
	setCatalog(t, sampleCatalog)

	t.Run("ok when OS matches", func(t *testing.T) {
		got, err := Resolve([]string{"windows-virtio-drivers"}, "windows-server-2022")
		if err != nil || len(got) != 1 {
			t.Fatalf("unexpected: got=%+v err=%v", got, err)
		}
	})

	t.Run("unsupported OS rejected, naming id and preference", func(t *testing.T) {
		_, err := Resolve([]string{"windows-virtio-drivers"}, "ubuntu-24")
		if err == nil || !strings.HasPrefix(err.Error(), "invalid container disk") {
			t.Fatalf("want invalid-prefix error, got %v", err)
		}
		if !strings.Contains(err.Error(), "windows-virtio-drivers") || !strings.Contains(err.Error(), "ubuntu-24") {
			t.Fatalf("error should name the id and the preference, got %q", err.Error())
		}
	})

	t.Run("unknown id rejected, naming the offender", func(t *testing.T) {
		_, err := Resolve([]string{"nope"}, "windows")
		if err == nil || !strings.HasPrefix(err.Error(), "invalid container disk") {
			t.Fatalf("want invalid-prefix error, got %v", err)
		}
		if !strings.Contains(err.Error(), "nope") {
			t.Fatalf("error should name the offending id, got %q", err.Error())
		}
	})

	t.Run("empty input returns empty slice, no error", func(t *testing.T) {
		got, err := Resolve([]string{}, "windows-server-2022")
		if err != nil || len(got) != 0 {
			t.Fatalf("want empty/no-error, got=%+v err=%v", got, err)
		}
	})

	t.Run("second id unknown short-circuits with error", func(t *testing.T) {
		_, err := Resolve([]string{"windows-virtio-drivers", "nope"}, "windows-server-2022")
		if err == nil || !strings.HasPrefix(err.Error(), "invalid container disk") {
			t.Fatalf("want invalid-prefix error, got %v", err)
		}
		if !strings.Contains(err.Error(), "nope") {
			t.Fatalf("error should name the offending id, got %q", err.Error())
		}
	})
}

func TestRecommendedFor(t *testing.T) {
	setCatalog(t, sampleCatalog)

	t.Run("returns only recommended matching the preference", func(t *testing.T) {
		got := RecommendedFor("windows-server-2022")
		if len(got) != 1 || got[0].ID != "windows-virtio-drivers" {
			t.Fatalf("unexpected: %+v", got)
		}
	})

	t.Run("empty when preference matches nothing recommended", func(t *testing.T) {
		// extra-tools matches ubuntu but is not recommended.
		if got := RecommendedFor("ubuntu-24"); len(got) != 0 {
			t.Fatalf("want empty, got %+v", got)
		}
	})

	t.Run("empty preference matches nothing", func(t *testing.T) {
		if got := RecommendedFor(""); len(got) != 0 {
			t.Fatalf("want empty, got %+v", got)
		}
	})

	t.Run("unknown preference returns nothing", func(t *testing.T) {
		if got := RecommendedFor("freebsd-13"); len(got) != 0 {
			t.Fatalf("want empty, got %+v", got)
		}
	})
}

func TestCatalogList(t *testing.T) {
	setCatalog(t, sampleCatalog)
	got := CatalogList()
	if len(got) != 2 {
		t.Fatalf("want 2 entries, got %d", len(got))
	}
	if got[0].ID != "windows-virtio-drivers" || got[0].Image != "quay.io/kubevirt/virtio-container-disk:v1.7.0" {
		t.Fatalf("unexpected entry: %+v", got[0])
	}
}

func TestCatalogListNilIsEmptySlice(t *testing.T) {
	setCatalog(t, nil)
	if got := CatalogList(); got == nil || len(got) != 0 {
		t.Fatalf("want non-nil empty slice, got %#v", got)
	}
}
