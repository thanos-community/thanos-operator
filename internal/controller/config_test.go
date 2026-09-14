package controller

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/thanos-community/thanos-operator/internal/pkg/featuregate"
)

func TestCacheOptionsForNamespace(t *testing.T) {
	all, err := CacheOptionsForNamespace("", featuregate.Config{})
	assert.NilError(t, err)
	assert.Assert(t, all.DefaultNamespaces == nil)

	scoped, err := CacheOptionsForNamespace("thanos-monitoring", featuregate.Config{})
	assert.NilError(t, err)
	assert.Equal(t, len(scoped.DefaultNamespaces), 1)
	_, exists := scoped.DefaultNamespaces["thanos-monitoring"]
	assert.Assert(t, exists)

	for _, namespace := range []string{"Monitoring", "bad_namespace", "a,b", " ", "$(POD_NAMESPACE)", strings.Repeat("a", 64)} {
		t.Run(namespace, func(t *testing.T) {
			_, err := CacheOptionsForNamespace(namespace, featuregate.Config{})
			assert.ErrorContains(t, err, "invalid watch namespace")
		})
	}
}
