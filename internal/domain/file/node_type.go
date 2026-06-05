package file

import "strings"

// Node types stored in file_node.node_type (aligned with wedata FileType where applicable).
const (
	NodeTypeFile      = "file"
	NodeTypeDirectory = "directory"
	NodeTypeGitFolder = "git_folder"
	NodeTypeNotebook  = "notebook"
)

// NormalizeNodeType returns a known node type or empty if invalid.
func NormalizeNodeType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case NodeTypeFile:
		return NodeTypeFile
	case NodeTypeDirectory:
		return NodeTypeDirectory
	case NodeTypeGitFolder:
		return NodeTypeGitFolder
	case NodeTypeNotebook:
		return NodeTypeNotebook
	default:
		return ""
	}
}

// NodeTypeFromDir reports directory vs file from a directory bit (not git_folder / notebook).
func NodeTypeFromDir(isDir bool) string {
	if isDir {
		return NodeTypeDirectory
	}
	return NodeTypeFile
}

// InferNodeTypeFromName guesses node type from basename when file_node has no row.
func InferNodeTypeFromName(name string, isDir bool) string {
	if isDir {
		return NodeTypeDirectory
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), ".ipynb") {
		return NodeTypeNotebook
	}
	return NodeTypeFile
}
