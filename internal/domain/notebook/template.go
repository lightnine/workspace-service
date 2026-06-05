package notebook

import (
	"encoding/json"
	"strings"
)

// DefaultKernelName is used when CreateNotebook omits kernel_name.
const DefaultKernelName = "python3"

// DefaultNotebook returns a minimal Jupyter nbformat v4 notebook (one empty code cell).
func DefaultNotebook(kernelName string) []byte {
	kernelName = strings.TrimSpace(kernelName)
	if kernelName == "" {
		kernelName = DefaultKernelName
	}
	displayName := "Python 3"
	if kernelName != DefaultKernelName {
		displayName = kernelName
	}

	doc := map[string]any{
		"cells": []map[string]any{
			{
				"cell_type":       "code",
				"execution_count": nil,
				"metadata":        map[string]any{},
				"outputs":         []any{},
				"source":          []string{},
			},
		},
		"metadata": map[string]any{
			"kernelspec": map[string]any{
				"display_name": displayName,
				"language":     "python",
				"name":         kernelName,
			},
			"language_info": map[string]any{
				"name":    "python",
				"version": "3.10.0",
			},
		},
		"nbformat":       4,
		"nbformat_minor": 5,
	}

	b, err := json.Marshal(doc)
	if err != nil {
		// static structure; unreachable
		return []byte(`{"cells":[],"metadata":{},"nbformat":4,"nbformat_minor":5}`)
	}
	return b
}
