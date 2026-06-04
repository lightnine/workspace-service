package req

type CreateFolderReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type CreateFileReq struct {
	WorkspaceContext
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
	Overwrite     bool   `json:"overwrite"`
}

type DeletePathReq struct {
	WorkspaceContext
	Path       string `json:"path"`
	SoftDelete bool   `json:"soft_delete"`
	Permanent  bool   `json:"permanent"`
}

type MovePathReq struct {
	WorkspaceContext
	SrcPath   string `json:"src_path"`
	DestPath  string `json:"dest_path"`
	Overwrite bool   `json:"overwrite"`
}

type CopyPathReq struct {
	WorkspaceContext
	SrcPath   string `json:"src_path"`
	DestPath  string `json:"dest_path"`
	Overwrite bool   `json:"overwrite"`
}

type RenamePathReq struct {
	WorkspaceContext
	Path      string `json:"path"`
	NewName   string `json:"new_name"`
	Overwrite bool   `json:"overwrite"`
}

type ListFilesReq struct {
	WorkspaceContext
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

type GetFileInfoReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type ReadFileReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type WriteFileReq struct {
	WorkspaceContext
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
	Overwrite     bool   `json:"overwrite"`
}

type DownloadFileReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type ValidatePathReq struct {
	WorkspaceContext
	ParentPath string `json:"parent_path"`
	Name       string `json:"name"`
}

type GetFolderNodePathReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type ListRecycleBinReq struct {
	WorkspaceContext
}

type RestorePathReq struct {
	WorkspaceContext
	TrashPath    string `json:"trash_path"`
	TargetParent string `json:"target_parent"`
}

type EmptyRecycleBinReq struct {
	WorkspaceContext
}
