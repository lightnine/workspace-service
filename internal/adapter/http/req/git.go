package req

type CloneRepositoryReq struct {
	WorkspaceContext
	RepoURL    string `json:"repo_url"`
	TargetPath string `json:"target_path"`
	Branch     string `json:"branch"`
}

type PullRepoReq struct {
	WorkspaceContext
	Path        string `json:"path"`
	RemoteName  string `json:"remote_name"`
	Branch      string `json:"branch"`
	GitUsername string `json:"git_username"`
	GitToken    string `json:"git_token"`
}

type CommitAndPushReq struct {
	WorkspaceContext
	Path        string `json:"path"`
	Message     string `json:"message"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
	RemoteName  string `json:"remote_name"`
	Push        *bool  `json:"push"`
	GitUsername string `json:"git_username"`
	GitToken    string `json:"git_token"`
}

type CreateBranchReq struct {
	WorkspaceContext
	Path     string `json:"path"`
	Branch   string `json:"branch"`
	Checkout bool   `json:"checkout"`
}

type CheckoutBranchReq struct {
	WorkspaceContext
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Create bool   `json:"create"`
}

type ListBranchesReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type GetStatusReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type GetFileDiffReq struct {
	WorkspaceContext
	Path string `json:"path"`
	File string `json:"file"`
}

type GetCommitHistoryReq struct {
	WorkspaceContext
	Path  string `json:"path"`
	Limit int    `json:"limit"`
}

type DiscardChangesReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type DeleteRepoReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type CreateGitFolderReq struct {
	WorkspaceContext
	TargetPath  string `json:"target_path"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	GitUsername string `json:"git_username"`
	GitToken    string `json:"git_token"`
}

type GetGitFolderStatusReq struct {
	WorkspaceContext
	Path string `json:"path"`
}

type StageFilesReq struct {
	WorkspaceContext
	Path  string   `json:"path"`
	Files []string `json:"files"`
	All   bool     `json:"all"`
}

type UnstageFilesReq struct {
	WorkspaceContext
	Path  string   `json:"path"`
	Files []string `json:"files"`
	All   bool     `json:"all"`
}

type CommitReq struct {
	WorkspaceContext
	Path        string `json:"path"`
	Message     string `json:"message"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
}

type PushRepoReq struct {
	WorkspaceContext
	Path        string `json:"path"`
	RemoteName  string `json:"remote_name"`
	Branch      string `json:"branch"`
	GitUsername string `json:"git_username"`
	GitToken    string `json:"git_token"`
}
