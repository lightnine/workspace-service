package fs

import "os"

// InodeInspector resolves the JuiceFS inode of an absolute mount path. The
// recovery scan uses it to re-stat an intent's target after a crash:
// existence (with a resolvable inode) is the source of truth for "the storage
// write landed". CreateFile writes atomically (temp + rename) so a path that
// exists is guaranteed to be a complete file.
type InodeInspector struct{}

func NewInodeInspector() InodeInspector {
	return InodeInspector{}
}

// StatInode reports the inode id of absPath. exists is false when the path is
// absent; err is only non-nil for unexpected stat failures.
func (InodeInspector) StatInode(absPath string) (inode uint64, exists bool, err error) {
	info, statErr := os.Stat(absPath)
	if os.IsNotExist(statErr) {
		return 0, false, nil
	}
	if statErr != nil {
		return 0, false, statErr
	}
	id, ok := inodeFromFileInfo(info)
	if !ok {
		return 0, true, nil
	}
	return id, true, nil
}
