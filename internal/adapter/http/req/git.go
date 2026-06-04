package req

type CloneRepositoryReq struct {
	OwnerUIN   string `json:"owner_uin"`
	UIN        string `json:"uin"`
	RepoURL    string `json:"repo_url"`
	TargetPath string `json:"target_path"`
	Branch     string `json:"branch"`
}
