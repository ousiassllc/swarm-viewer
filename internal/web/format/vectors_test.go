package format_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ousiassllc/swarm-viewer/internal/domain"
	"github.com/ousiassllc/swarm-viewer/internal/web/format"
)

// minStateLabels は testdata/vectors/report.json の state_labels の件数の下限（ファイルが空でも空振りしないため）。
const minStateLabels = 9

// state の表示ラベル（機能要件 §3）。Android 版（Kotlin）と同じ testdata/vectors/report.json を読む。
func TestVectorsStateLabels(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "vectors", "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		StateLabels []struct {
			State string `json:"state"`
			Label string `json:"label"`
		} `json:"state_labels"`
	}
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&f); err != nil {
		t.Fatal(err)
	}
	if len(f.StateLabels) < minStateLabels {
		t.Fatalf("state_labels が %d 件しかない（下限 %d）", len(f.StateLabels), minStateLabels)
	}
	for _, c := range f.StateLabels {
		if got := format.StateLabel(domain.ParsePaneState(c.State)); got != c.Label {
			t.Errorf("StateLabel(%q) = %q; want %q", c.State, got, c.Label)
		}
	}
}
