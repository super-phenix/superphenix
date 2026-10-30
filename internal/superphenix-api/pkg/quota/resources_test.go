package quota

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const gi = int64(1024 * 1024 * 1024)

func TestParse(t *testing.T) {
	t.Run("allowed resources", func(t *testing.T) {
		q, err := Resources{CPU: "2500m", Memory: "64Gi", Disk: "1Ti"}.Parse()
		require.NoError(t, err)
		assert.Equal(t, Quantities{CPU: 2500, Memory: 64 * gi, Disk: 1024 * gi}, q)
	})

	invalid := map[string]Resources{
		"invalid quantity":        {CPU: "invalid"},
		"negative quantity":       {Memory: "-1"},
		"too large quantity":      {Memory: "100Ei"},
		"too large cpu":           {CPU: "10E15"},
		"empty quantity":          {CPU: ""},
		"translated key":          {"requests.cpu": "4"},
		"translated disk key":     {"requests.storage": "4Gi"},
		"pods":                    {"pods": "10"},
		"count resource":          {"count/virtualmachines.kubevirt.io": "20"},
		"extended resource":       {"requests.nvidia.com/gpu": "0"},
		"former storage resource": {"storage": "1Ti"},
		"yaml injection in name":  {"pods: 1\n  limits.cpu": "1"},
	}
	for name, r := range invalid {
		t.Run(name, func(t *testing.T) {
			_, err := r.Parse()
			assert.ErrorIs(t, err, ErrInvalidResources)
		})
	}

	t.Run("unsupported resource error lists the allowed ones", func(t *testing.T) {
		_, err := Resources{"pods": "1"}.Parse()
		assert.ErrorContains(t, err, "cpu, disk, memory")
	})
}

func TestToKubernetesQuota(t *testing.T) {
	hard := Resources{CPU: "16", Memory: "64Gi", Disk: "0"}.ToKubernetesQuota()
	assert.Equal(t, map[string]string{
		"requests.cpu":     "16",
		"limits.cpu":       "16",
		"requests.memory":  "64Gi",
		"limits.memory":    "64Gi",
		"requests.storage": "0",
	}, hard)
}

func TestQuantitiesResources(t *testing.T) {
	assert.Equal(t, Resources{CPU: "16", Memory: "64Gi", Disk: "0"},
		Quantities{CPU: 16000, Memory: 64 * gi, Disk: 0}.Resources())
	assert.Equal(t, Resources{CPU: "3500m", Memory: "500Mi"},
		Quantities{CPU: 3500, Memory: 500 * 1024 * 1024}.Resources())
}

func TestSplitEqually(t *testing.T) {
	orig := Quantities{CPU: 10000, Memory: 300 * gi, Disk: 2}

	split := orig.SplitEqually(3)
	assert.Equal(t, Quantities{CPU: 3333, Memory: 100 * gi, Disk: 0}, split)
	_, exceeded := split.Add(split).Add(split).Exceeds(orig)
	assert.False(t, exceeded, "the parts never exceed the whole")

	// A resource split down to 0 stays limited (at 0), it does not become unlimited.
	assert.Equal(t, "0", split.Resources()[Disk])

	assert.Empty(t, orig.SplitEqually(0))
	assert.Empty(t, orig.SplitEqually(-1))
}

func TestAddSub(t *testing.T) {
	a := Quantities{CPU: 1000, Disk: 10}
	b := Quantities{CPU: 500, Memory: gi}
	assert.Equal(t, Quantities{CPU: 1500, Memory: gi, Disk: 10}, a.Add(b))
	assert.Equal(t, Quantities{CPU: 500, Disk: 10}, a.Sub(b))
	assert.Equal(t, Quantities{CPU: 0, Memory: gi}, b.Sub(Quantities{CPU: 2000}))

	// Sums saturate instead of overflowing into negative (i.e. passing) values.
	huge := Quantities{Disk: 1 << 62}
	assert.Equal(t, int64(1<<63-1), huge.Add(huge).Add(huge)[Disk])
}

func TestExceeds(t *testing.T) {
	limit := Quantities{CPU: 16000, Disk: 0}

	_, exceeded := Quantities{CPU: 16000}.Exceeds(limit)
	assert.False(t, exceeded, "missing resources count as 0, equal is allowed")

	_, exceeded = Quantities{Memory: 1000 * gi}.Exceeds(limit)
	assert.False(t, exceeded, "resources not limited are ignored")

	msg, exceeded := Quantities{CPU: 17000}.Exceeds(limit)
	assert.True(t, exceeded)
	assert.Equal(t, "cpu quota exceeded: requested 17 exceeds limit 16", msg)

	msg, exceeded = Quantities{Disk: 1}.Exceeds(limit)
	assert.True(t, exceeded, "a resource limited to 0 is enforced")
	assert.Contains(t, msg, Disk)
}

func TestEffectiveAZQuota(t *testing.T) {
	az := Resources{CPU: "4", Disk: "10Gi"}

	assert.Equal(t, az, EffectiveAZQuota(nil, nil, az), "an AZ quota is enforced without any ancestor quota")
	assert.Empty(t, EffectiveAZQuota(nil, nil, nil))

	assert.Equal(t, Resources{CPU: "4", Disk: "10Gi"},
		EffectiveAZQuota(nil, Resources{CPU: "2"}, az),
		"an overcommitted AZ quota is enforced as is")

	assert.Equal(t, Resources{CPU: "4", Disk: "10Gi", Memory: "0"},
		EffectiveAZQuota(nil, Resources{CPU: "8", Memory: "8Gi"}, az),
		"resources limited by the project but not allocated to the AZ are 0")

	assert.Equal(t, Resources{CPU: "0", Memory: "0"},
		EffectiveAZQuota(Resources{CPU: "8", Memory: "8Gi"}, nil, nil),
		"a project without quota in a limited organization gets nothing")

	assert.Equal(t, Resources{CPU: "4", Disk: "10Gi", Memory: "0"},
		EffectiveAZQuota(Resources{Memory: "8Gi"}, Resources{CPU: "8"}, az),
		"resources limited by the organization are enforced even if the project quota predates them")
}

func TestFromKubernetesUsage(t *testing.T) {
	got := FromKubernetesUsage(map[string]string{
		"requests.cpu":     "1500m",
		"limits.cpu":       "2",
		"requests.memory":  "1Gi",
		"requests.storage": "10Gi",
		"pods":             "3",
		"limits.memory":    "invalid",
	})
	want := Quantities{CPU: 2000, Memory: 1 << 30, Disk: 10 << 30}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for name, v := range want {
		if got[name] != v {
			t.Fatalf("%s: got %d, want %d", name, got[name], v)
		}
	}
	if len(FromKubernetesUsage(nil)) != 0 {
		t.Fatal("expected no usage")
	}
}
