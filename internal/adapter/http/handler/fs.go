package handler

import (
	"errors"
	"net/http"

	"git.woa.com/leondli/workspace-service/internal/adapter/http/req"
	httpresponse "git.woa.com/leondli/workspace-service/internal/adapter/http/response"
	domainfs "git.woa.com/leondli/workspace-service/internal/domain/fs"
	usecasefs "git.woa.com/leondli/workspace-service/internal/usecase/fs"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	fileService usecasefs.FileService
}

func NewFileHandler(fileService usecasefs.FileService) *FileHandler {
	return &FileHandler{fileService: fileService}
}

func (h *FileHandler) CreateFolder(c *gin.Context) {
	var body req.CreateFolderReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.CreateFolder(c.Request.Context(), usecasefs.CreateFolderReq{
		Context: rc, Path: body.Path,
	})
	writeFSResult(c, out, err)
}

func (h *FileHandler) CreateFile(c *gin.Context) {
	var body req.CreateFileReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.CreateFile(c.Request.Context(), usecasefs.CreateFileReq{
		Context: rc, Path: body.Path,
		ContentB64: body.ContentBase64, Overwrite: body.Overwrite,
	})
	writeFSResult(c, out, err)
}

func (h *FileHandler) DeletePath(c *gin.Context) {
	var body req.DeletePathReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	err := h.fileService.DeletePath(c.Request.Context(), usecasefs.DeletePathReq{
		Context: rc, Path: body.Path,
		SoftDelete: body.SoftDelete,
		Permanent:  body.Permanent,
	})
	if err != nil {
		writeFSError(c, err)
		return
	}
	httpresponse.OK(c, gin.H{"deleted": true})
}

func (h *FileHandler) MovePath(c *gin.Context) {
	var body req.MovePathReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.MovePath(c.Request.Context(), usecasefs.MovePathReq{
		Context: rc, SrcPath: body.SrcPath, DestPath: body.DestPath, Overwrite: body.Overwrite,
	})
	writeFSResult(c, out, err)
}

func (h *FileHandler) CopyPath(c *gin.Context) {
	var body req.CopyPathReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.CopyPath(c.Request.Context(), usecasefs.CopyPathReq{
		Context: rc, SrcPath: body.SrcPath, DestPath: body.DestPath, Overwrite: body.Overwrite,
	})
	writeFSResult(c, out, err)
}

func (h *FileHandler) RenamePath(c *gin.Context) {
	var body req.RenamePathReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.RenamePath(c.Request.Context(), usecasefs.RenamePathReq{
		Context: rc, Path: body.Path, NewName: body.NewName, Overwrite: body.Overwrite,
	})
	writeFSResult(c, out, err)
}

func (h *FileHandler) ListFiles(c *gin.Context) {
	var body req.ListFilesReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.ListFiles(c.Request.Context(), usecasefs.ListFilesReq{
		Context: rc, Path: body.Path, Recursive: body.Recursive,
	})
	if err != nil {
		writeFSError(c, err)
		return
	}
	httpresponse.OK(c, out)
}

func (h *FileHandler) GetFileInfo(c *gin.Context) {
	var body req.GetFileInfoReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.GetFileInfo(c.Request.Context(), usecasefs.GetFileInfoReq{
		Context: rc, Path: body.Path,
	})
	writeFSResult(c, out, err)
}

func (h *FileHandler) ReadFile(c *gin.Context) {
	var body req.ReadFileReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.ReadFile(c.Request.Context(), usecasefs.ReadFileReq{
		Context: rc, Path: body.Path,
	})
	if err != nil {
		writeFSError(c, err)
		return
	}
	httpresponse.OK(c, out)
}

func (h *FileHandler) WriteFile(c *gin.Context) {
	var body req.WriteFileReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.WriteFile(c.Request.Context(), usecasefs.WriteFileReq{
		Context: rc, Path: body.Path,
		ContentB64: body.ContentBase64, Overwrite: body.Overwrite,
	})
	writeFSResult(c, out, err)
}

func (h *FileHandler) ValidatePath(c *gin.Context) {
	var body req.ValidatePathReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.ValidatePath(c.Request.Context(), usecasefs.ValidatePathReq{
		Context: rc, ParentPath: body.ParentPath, Name: body.Name,
	})
	if err != nil {
		writeFSError(c, err)
		return
	}
	httpresponse.OK(c, out)
}

func (h *FileHandler) GetFolderNodePath(c *gin.Context) {
	var body req.GetFolderNodePathReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.GetFolderNodePath(c.Request.Context(), usecasefs.GetFolderNodePathReq{
		Context: rc, Path: body.Path,
	})
	if err != nil {
		writeFSError(c, err)
		return
	}
	httpresponse.OK(c, out)
}

func (h *FileHandler) ListRecycleBin(c *gin.Context) {
	var body req.ListRecycleBinReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.ListRecycleBin(c.Request.Context(), usecasefs.ListRecycleBinReq{Context: rc})
	if err != nil {
		writeFSError(c, err)
		return
	}
	httpresponse.OK(c, out)
}

func (h *FileHandler) RestorePath(c *gin.Context) {
	var body req.RestorePathReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	out, err := h.fileService.RestorePath(c.Request.Context(), usecasefs.RestorePathReq{
		Context: rc, TrashPath: body.TrashPath, TargetParent: body.TargetParent,
	})
	writeFSResult(c, out, err)
}

func (h *FileHandler) EmptyRecycleBin(c *gin.Context) {
	var body req.EmptyRecycleBinReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	err := h.fileService.EmptyRecycleBin(c.Request.Context(), usecasefs.EmptyRecycleBinReq{Context: rc})
	if err != nil {
		writeFSError(c, err)
		return
	}
	httpresponse.OK(c, gin.H{"emptied": true})
}

func (h *FileHandler) DownloadFile(c *gin.Context) {
	var body req.DownloadFileReq
	if !bindFSJSON(c, &body) {
		return
	}
	rc, ok := bindWorkspaceContext(c, &body.WorkspaceContext)
	if !ok {
		return
	}
	data, name, err := h.fileService.DownloadFile(c.Request.Context(), usecasefs.ReadFileReq{
		Context: rc, Path: body.Path,
	})
	if err != nil {
		writeFSError(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+name)
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func bindFSJSON(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		httpresponse.Error(c, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeFSResult(c *gin.Context, data usecasefs.FileInfoResp, err error) {
	if err != nil {
		writeFSError(c, err)
		return
	}
	httpresponse.OK(c, data)
}

func writeFSError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, usecasefs.ErrInvalidFileRequest), req.IsInvalidContext(err):
		httpresponse.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, domainfs.ErrNotFound):
		httpresponse.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, domainfs.ErrAlreadyExists):
		httpresponse.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, domainfs.ErrNotAFile), errors.Is(err, domainfs.ErrNotADirectory):
		httpresponse.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, domainfs.ErrFileTooLarge):
		httpresponse.Error(c, http.StatusRequestEntityTooLarge, err.Error())
	default:
		httpresponse.Error(c, http.StatusInternalServerError, err.Error())
	}
}
