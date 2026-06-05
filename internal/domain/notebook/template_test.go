package notebook

import (
	"encoding/json"
	"testing"
)

func TestDefaultNotebook(t *testing.T) {
	t.Parallel()
	raw := DefaultNotebook("")
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if doc["nbformat"].(float64) != 4 {
		t.Fatalf("nbformat = %v", doc["nbformat"])
	}
	cells, ok := doc["cells"].([]any)
	if !ok || len(cells) != 1 {
		t.Fatalf("cells = %v", doc["cells"])
	}
	meta, ok := doc["metadata"].(map[string]any)
	if !ok {
		t.Fatal("missing metadata")
	}
	ks, ok := meta["kernelspec"].(map[string]any)
	if !ok || ks["name"] != DefaultKernelName {
		t.Fatalf("kernelspec = %v", meta["kernelspec"])
	}
}
