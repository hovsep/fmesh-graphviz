package dot

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Run `go test ./dot -update` to rewrite the golden files after an intended
// change to the output, then review the diff.
var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// assertGolden compares got with testdata/<name>.golden.
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o750))
		require.NoError(t, os.WriteFile(path, got, 0o600))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden file; run go test ./dot -update")
	assert.Equal(t, string(want), string(got))
}

func TestExporter_Golden(t *testing.T) {
	// The DOT output is long (HTML legend, attributes), so it is pinned in
	// golden files instead of inline strings.
	fm := calcMesh(t)
	e := New()

	static, err := e.Export(fm)
	require.NoError(t, err)
	assertGolden(t, "calc-static", static)

	for i, graph := range runAndExportCycles(t, e, fm) {
		assertGolden(t, fmt.Sprintf("calc-cycle-%d", i+1), []byte(graph))
	}
}
