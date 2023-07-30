package service

import (
	"bytes"
)

type File struct {
	Path   string
	buffer *bytes.Buffer
}

func NewFile() *File {
	return &File{buffer: &bytes.Buffer{}}
}
