package process

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseJobConfig(t *testing.T) {
	path := "./testdata/config.yaml"

	cfg := ParseJobConfig(path)

	require.Equal(t, 2, len(cfg.Jobs))
	require.Equal(t, "interceptor", cfg.Jobs[0].Name)
	require.Equal(t, "/app/interceptor", cfg.Jobs[0].BinaryPath)
	require.Equal(t, "interceptor", cfg.Jobs[0].ServiceName)
	require.Equal(t, "customer", cfg.Jobs[1].Name)
	require.Equal(t, "/app/customer", cfg.Jobs[1].BinaryPath)
	require.Equal(t, "customer", cfg.Jobs[1].ServiceName)
}
