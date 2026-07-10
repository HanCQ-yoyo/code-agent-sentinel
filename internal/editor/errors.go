package editor

import "errors"

// 编辑流程错误。handler 据此映射 HTTP code。
var (
	ErrConcurrentModification = errors.New("editor: concurrent modification (base hash mismatch)")
	ErrNotEditable            = errors.New("editor: asset not editable")
	ErrOutOfRoot              = errors.New("editor: source path out of allowed root")
	ErrBadContent             = errors.New("editor: bad content")
)
