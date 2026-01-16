/* Copyright(C) 2023. Huawei Technologies Co.,Ltd. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package utils provides the util func
package utils

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const (
	// FileMode file privilege
	FileMode = 0600
	// Size10M  bytes of 10M
	Size10M = 10 * 1024 * 1024
	maxSize = 1024 * 1024 * 1024
)

// ReadLimitBytes read limit length of contents from file path
func ReadLimitBytes(path string, limitLength int) ([]byte, error) {
	if limitLength < 0 || limitLength > maxSize {
		return nil, errors.New("the limit length is not valid")
	}

	key, err := CheckPath(path)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(key, os.O_RDONLY, FileMode)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("open file with read-only and %04o mode failed", FileMode))
	}
	defer file.Close()
	buf := make([]byte, limitLength, limitLength)
	l, err := file.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %v", err)
	}
	return buf[0:l], nil
}

// LoadFile load file content
func LoadFile(filePath string) ([]byte, error) {
	if filePath == "" {
		return nil, nil
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("the filePath is invalid: %v", err)
	}
	if !IsExist(absPath) {
		return nil, nil
	}

	return ReadLimitBytes(absPath, Size10M)
}

func closeFile(file *os.File) {
	if file == nil {
		return
	}
	if err := file.Close(); err != nil {
		return
	}
	return
}

// CopyFile copy file
func CopyFile(src, dst string) error {
	src, err := CheckPath(src)
	if err != nil {
		return err
	}
	if IsExist(dst) {
		dst, err = CheckPath(dst)
		if err != nil {
			return err
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer closeFile(srcFile)

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer closeFile(dstFile)

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// CopyDir recursively copy files
func CopyDir(src string, dst string) error {
	var (
		err     error
		fds     []os.FileInfo = nil
		dstInfo os.FileInfo
	)

	if dstInfo, err = os.Stat(src); err != nil {
		return err
	}
	if err = os.MkdirAll(dst, dstInfo.Mode()); err != nil {
		return err
	}
	if subFolder(src, dst) {
		return errors.New("the destination directory is a subdirectory of the source directory")
	}
	if fds, err = ioutil.ReadDir(src); err != nil {
		return err
	}
	for _, fd := range fds {
		srcFile := filepath.Join(src, fd.Name())
		dstFile := filepath.Join(dst, fd.Name())
		if fd.IsDir() {
			if err = CopyDir(srcFile, dstFile); err != nil {
				return err
			}
		} else {
			if err = CopyFile(srcFile, dstFile); err != nil {
				return err
			}
		}
	}
	return nil
}

func subFolder(src, dst string) bool {
	if src == dst {
		return true
	}
	srcReal, err := filepath.EvalSymlinks(src)
	if err != nil {
		return false
	}
	dstReal, err := filepath.EvalSymlinks(dst)
	if err != nil {
		return false
	}
	srcList := strings.Split(srcReal, string(os.PathSeparator))
	dstList := strings.Split(dstReal, string(os.PathSeparator))
	if len(srcList) > len(dstList) {
		return false
	}
	return reflect.DeepEqual(srcList, dstList[:len(srcList)])
}
