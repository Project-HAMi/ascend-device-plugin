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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	notValidPath           = "not-valid-file-path"
	maxAllowFileSize int64 = 1024 * 100 // in megabytes
	oneMegabytes     int64 = 1024 * 1024
	// DefaultWhiteList default white list in string
	DefaultWhiteList = "-_./~"
	// DefaultStringLength default string max length
	DefaultStringLength = 256
	// DefaultPathLength default path max length
	DefaultPathLength = 4096
)

// RealFileChecker Check whether the file is valid
func RealFileChecker(path string, checkParent, allowLink bool, size int64) (string, error) {
	realPath, fileInfo, err := realPathChecker(path, checkParent, allowLink)
	if err != nil {
		return notValidPath, err
	}
	if fileInfo.IsDir() {
		return notValidPath, fmt.Errorf("invalid dir")
	}
	if !fileInfo.Mode().IsRegular() {
		return notValidPath, fmt.Errorf("invalid regular file")
	}
	if size > maxAllowFileSize || size < 0 {
		return notValidPath, fmt.Errorf("invalid size")
	}
	if fileInfo.Size() > size*oneMegabytes {
		return notValidPath, fmt.Errorf("size too large")
	}
	return realPath, nil
}

// RealDirChecker Check whether the directory is valid
func RealDirChecker(path string, checkParent, allowLink bool) (string, error) {
	realPath, fileInfo, err := realPathChecker(path, checkParent, allowLink)
	if err != nil {
		return notValidPath, err
	}
	if !fileInfo.IsDir() {
		return notValidPath, fmt.Errorf("is not dir")
	}
	return realPath, nil
}

// PathStringChecker Check whether the directory string is valid
func PathStringChecker(path string) (string, error) {
	realPath, err := filepath.Abs(path)
	if err != nil {
		return notValidPath, err
	}
	if len(realPath) > DefaultPathLength {
		return notValidPath, fmt.Errorf("path over max path length")
	}
	if !stringChecker(realPath, 0, DefaultPathLength) {
		return notValidPath, fmt.Errorf("invalid path")
	}
	if err = pathDepthChecker(realPath, 0); err != nil {
		return notValidPath, err
	}
	return realPath, nil
}

// VerifyFile verify the file after it is opened.
func VerifyFile(file *os.File, size int64) error {
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	if size > maxAllowFileSize || size < 0 {
		return fmt.Errorf("invalid size")
	}
	if fileInfo.Size() > size*oneMegabytes {
		return fmt.Errorf("file size error %v", fileInfo.Size())
	}
	if (fileInfo.Mode() & fs.ModeSymlink) != 0 {
		return fmt.Errorf("file is softlink")
	}
	if st := fileInfo.Sys(); st.(*syscall.Stat_t).Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("file owner incorrect")
	}
	return nil
}

// SafeChmod after the verification is complete, run the chmod command.
func SafeChmod(path string, size int64, mode os.FileMode) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if err = VerifyFile(file, size); err != nil {
		return err
	}
	if err = file.Chmod(mode); err != nil {
		return err
	}
	return nil
}

func realPathChecker(path string, checkParent, allowLink bool) (string, os.FileInfo, error) {
	realPath, err := filepath.Abs(path)
	if err != nil {
		return notValidPath, nil, err
	}
	if len(realPath) > DefaultPathLength {
		return notValidPath, nil, fmt.Errorf("path over max path length")
	}
	if !stringChecker(realPath, 0, DefaultPathLength) {
		return notValidPath, nil, fmt.Errorf("invalid path")
	}
	if err = fileChecker(realPath, true, checkParent, allowLink, 0); err != nil {
		return notValidPath, nil, err
	}
	fileInfo, err := os.Stat(realPath)
	if err != nil {
		return notValidPath, nil, err
	}
	return realPath, fileInfo, nil
}

func fileChecker(path string, allowDir, checkParent, allowLink bool, deep int) error {
	const maxDepth int = 99
	if deep > maxDepth {
		return fmt.Errorf("over maxDepth %d", maxDepth)
	}
	fileInfo, err := normalFileCheck(path, allowDir, allowLink)
	if err != nil {
		return err
	}
	if err = checkOwnerAndPermission(fileInfo, path); err != nil {
		return err
	}
	if path != "/" && checkParent {
		return fileChecker(filepath.Dir(path), true, true, allowLink, deep+1)
	}
	return nil
}

func pathDepthChecker(path string, deep int) error {
	const maxDepth int = 99
	if deep > maxDepth {
		return fmt.Errorf("over maxDepth %d", maxDepth)
	}
	if path != "/" {
		return pathDepthChecker(filepath.Dir(path), deep+1)
	}
	return nil
}

func checkOwnerAndPermission(fileInfo os.FileInfo, filePath string) error {
	const groupWriteIndex, otherWriteIndex, permLength int = 5, 8, 10
	perm := fileInfo.Mode().Perm().String()
	if len(perm) != permLength {
		return fmt.Errorf("permission not right %v %v", filePath, perm)
	}
	for index, char := range perm {
		if (index == groupWriteIndex || index == otherWriteIndex) && char == 'w' {
			return fmt.Errorf("write permission not right %v %v", filePath, perm)
		}
	}
	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("can not get stat %v", filePath)
	}
	if !(int(stat.Uid) == 0 || int(stat.Uid) == os.Getuid()) {
		return fmt.Errorf("owner not right %v %v", filePath, int(stat.Uid))
	}
	return nil
}

func normalFileCheck(filePath string, allowDir, allowLink bool) (os.FileInfo, error) {
	realPath, err := filepath.EvalSymlinks(filePath)
	if err != nil || (realPath != filePath && !allowLink) {
		return nil, fmt.Errorf("symlinks or not existed, failed %v, %v", filePath, err)
	}
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("get file stat failed %v", err)
	}
	if allowDir && !fileInfo.Mode().IsRegular() && !fileInfo.IsDir() {
		return nil, fmt.Errorf("not regular file/dir %v", filePath)
	}
	if !allowDir && !fileInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("not regular file %v", filePath)
	}
	if fileInfo.Mode()&os.ModeSetuid != 0 {
		return nil, fmt.Errorf("setuid not allowed %v", filePath)
	}
	if fileInfo.Mode()&os.ModeSetgid != 0 {
		return nil, fmt.Errorf("setgid not allowed %v", filePath)
	}
	return fileInfo, nil
}

func isValidCode(c rune) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9')
}

func isInWhiteList(c rune) bool {
	return strings.Contains(DefaultWhiteList, string(c))
}

func stringChecker(text string, minLength, maxLength int) bool {
	if len(text) <= minLength || len(text) >= maxLength {
		return false
	}
	for _, char := range text {
		if !isValidCode(char) && !isInWhiteList(char) {
			return false
		}
	}
	return true
}
