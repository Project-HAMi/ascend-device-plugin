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
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	dirMode = 0700

	rootUID       = 0
	maxPathDepth  = 20
	maxPathLength = 1024
	// DefaultWriteFileMode  default file mode for write permission check
	DefaultWriteFileMode = 0022

	ldSplitLen     = 2
	ldLibNameIndex = 0
	ldLibPathIndex = 1
	ldCommand      = "/sbin/ldconfig"
	ldParam        = "--print-cache"
	// LdLibPath LD_LIBRARY_PATH
	LdLibPath   = "LD_LIBRARY_PATH"
	grepCommand = "/bin/grep"
)

// IsDir check whether the path is a directory.
func IsDir(path string) bool {
	if path == "" {
		return false
	}

	if !IsExist(path) {
		return path[len(path)-1:] == "/"
	}
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

// IsFile check whether the path is a file
func IsFile(path string) bool {
	if path == "" {
		return false
	}
	return !IsDir(path)
}

// IsSoftlink check whether the path is softlink
func IsSoftlink(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return false, err
	}
	if (fileInfo.Mode() & fs.ModeSymlink) != 0 {
		return true, nil
	}
	return false, nil
}

// IsExist check whether the path exists, If the file is a symbolic link, the returned the final FileInfo
func IsExist(filePath string) bool {
	_, err := os.Stat(filePath)
	if err == nil {
		return true
	}
	if os.IsExist(err) {
		return true
	}
	return false
}

// IsLexist check whether the path exists, If the file is a symbolic link, the returned FileInfo
// describes the symbolic link
func IsLexist(filePath string) bool {
	_, err := os.Lstat(filePath)
	if err == nil {
		return true
	}
	if os.IsExist(err) {
		return true
	}
	return false
}

// CheckPath  validate given path and return resolved absolute path
func CheckPath(path string) (string, error) {
	if path == "" {
		return path, nil
	}
	origin := path
	for !IsLexist(path) {
		path = filepath.Dir(path)
		if path == "." {
			return "", os.ErrNotExist
		}
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("get the absolute path failed: %v", err)
	}
	resoledPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("get the symlinks path failed: %v", err)
	}
	if absPath != resoledPath {
		return "", errors.New("can't support symlinks")
	}
	// get the original full path
	absOrigin, err := filepath.Abs(origin)
	if err != nil {
		return "", fmt.Errorf("get the absolute path failed: %v", err)
	}
	return absOrigin, nil
}

// MakeSureDir create directory. The last element of path should end with slash, or it will be omitted.
func MakeSureDir(path string) error {
	dir := filepath.Dir(path)
	if IsExist(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("create directory failed: %v", err)
	}

	return nil
}

// CheckMode check input file mode whether includes invalid mode.
// For example, if read operation of group and other is forbidden, then call CheckMode(inputFileMode, 0044).
// All operations are forbidden for group and other, then call CheckMode(inputFileMode, 0077).
// Write operation is forbidden for group and other by default, with calling CheckMode(inputFileMode)
func CheckMode(mode os.FileMode, optional ...os.FileMode) bool {
	var targetMode os.FileMode
	if len(optional) > 0 {
		targetMode = optional[0]
	} else {
		targetMode = DefaultWriteFileMode
	}
	checkMode := uint32(mode) & uint32(targetMode)
	return checkMode == 0
}

// CheckOwnerAndPermission check path  owner and permission
func CheckOwnerAndPermission(verifyPath string, mode os.FileMode, uid uint32) (string, error) {
	if verifyPath == "" {
		return verifyPath, errors.New("empty path")
	}
	absPath, err := filepath.Abs(verifyPath)
	if err != nil {
		return "", fmt.Errorf("abs failed %v", err)
	}
	resoledPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("evalSymlinks failed %v", err)
	}
	// if symlinks
	if absPath != resoledPath {
		// check symlinks its self owner
		pathInfo, err := os.Lstat(absPath)
		if err != nil {
			return "", fmt.Errorf("lstat failed, %v", err)
		}
		stat, ok := pathInfo.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uid {
			return "", errors.New("symlinks owner may not root")
		}
	}
	pathInfo, err := os.Stat(resoledPath)
	if err != nil {
		return "", fmt.Errorf("stat failed %v", err)
	}
	stat, ok := pathInfo.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uid || !CheckMode(pathInfo.Mode(), mode) {
		return "", errors.New("check uid or mode failed")
	}
	return resoledPath, nil
}

// DoCheckOwnerAndPermission check path owner and permission
func DoCheckOwnerAndPermission(path string, mode os.FileMode, uid uint32) error {
	if !IsExist(path) {
		return nil
	}
	pathInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat failed %v", err)
	}
	stat, ok := pathInfo.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uid || !CheckMode(pathInfo.Mode(), mode) {
		return fmt.Errorf("check uid or mode failed : %v", path)
	}
	return nil
}

func checkAbsPath(libPath string) (string, error) {
	absLibPath, err := CheckOwnerAndPermission(libPath, DefaultWriteFileMode, rootUID)
	if err != nil {
		return "", fmt.Errorf("%s: %v", libPath, err)
	}
	count := 0
	fPath := absLibPath
	for {
		if count >= maxPathDepth {
			break
		}
		count++
		if fPath == "/" {
			return absLibPath, nil
		}
		fPath = filepath.Dir(fPath)
		if _, err := CheckOwnerAndPermission(fPath, DefaultWriteFileMode, rootUID); err != nil {
			return "", fmt.Errorf("%s: %v", fPath, err)
		}
	}
	return "", errors.New("absolute path check failed")
}

func checkLibsPath(libraryPaths []string) (string, error) {
	errs := make([]string, 0, len(libraryPaths))
	for _, libraryAbsName := range libraryPaths {
		absLibPath, err := checkAbsPath(libraryAbsName)
		if err == nil {
			return absLibPath, nil
		}
		errs = append(errs, fmt.Sprintf("%s;", err.Error()))
	}
	return "", fmt.Errorf("lib path is invalid, %v", errs)
}

func getLibFromEnv(libraryName string) (string, error) {
	ldLibraryPath := os.Getenv(LdLibPath)
	if len(ldLibraryPath) > maxPathLength {
		return "", fmt.Errorf("invalid library path env")
	}
	libraryPaths := strings.Split(ldLibraryPath, ":")
	targetLibs := make([]string, 0, len(ldLibraryPath))
	for _, libraryPath := range libraryPaths {
		libraryAbsName := path.Join(libraryPath, libraryName)
		if len(libraryAbsName) > maxPathLength || !IsLexist(libraryAbsName) {
			continue
		}
		targetLibs = append(targetLibs, libraryAbsName)
	}
	if len(libraryPaths) == 0 {
		return "", errors.New("file path no exist or too long")
	}
	return checkLibsPath(targetLibs)
}

func trimSpaceTable(data string) string {
	data = strings.Replace(data, " ", "", -1)
	data = strings.Replace(data, "\t", "", -1)
	data = strings.Replace(data, "\n", "", -1)
	return data
}

func parserLibPath(line, libraryName string) string {
	ldInfo := strings.Split(line, "=>")
	if len(ldInfo) < ldSplitLen {
		return ""
	}
	libNames := strings.Split(ldInfo[ldLibNameIndex], " ")
	for index, libName := range libNames {
		if index >= maxPathDepth {
			break
		}
		if len(libName) == 0 {
			continue
		}
		if name := trimSpaceTable(libName); name != libraryName {
			continue
		}
		return trimSpaceTable(ldInfo[ldLibPathIndex])
	}
	return ""
}

func parseLibFromLdCmd(libraryName string) (string, error) {
	ldCmd := exec.Command(ldCommand, ldParam)
	grepCmd := exec.Command(grepCommand, libraryName)
	ldCmdStdout, err := ldCmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("command exec failed: %v", err)
	}
	grepCmd.Stdin = ldCmdStdout
	stdout, err := grepCmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("get pipe failed: %v", err)
	}
	if err = grepCmd.Start(); err != nil {
		return "", fmt.Errorf("command exec failed: %v", err)
	}
	if err = ldCmd.Run(); err != nil {
		return "", fmt.Errorf("command exec failed: %v", err)
	}
	defer func() {
		if err = grepCmd.Wait(); err != nil {
			log.Printf("command exec failed, %v", err)
		}
	}()
	reader := bufio.NewReader(stdout)
	count := 0
	line := ""
	for {
		if count >= maxPathLength {
			err = errors.New("too many items in command stdout")
			break
		}
		count++
		line, err = reader.ReadString('\n')
		if err != nil || io.EOF == err {
			break
		}
		if libPath := parserLibPath(line, libraryName); libPath != "" {
			return libPath, nil
		}
	}
	return "", fmt.Errorf("can't find valid lib: %v", err)
}

func getLibFromLdCmd(libraryName string) (string, error) {
	libraryAbsName, err := parseLibFromLdCmd(libraryName)
	if err != nil {
		return "", err
	}
	var absLibPath string
	if absLibPath, err = checkAbsPath(libraryAbsName); err == nil {
		return absLibPath, nil
	}
	return "", fmt.Errorf("driver lib is not exist or it's permission is invalid, %v", err)
}

// GetDriverLibPath get driver lib path from ld config
func GetDriverLibPath(libraryName string) (string, error) {
	var libPath string
	var envErr, cmdErr error
	if libPath, envErr = getLibFromEnv(libraryName); envErr == nil {
		return libPath, nil
	}
	if libPath, cmdErr = getLibFromLdCmd(libraryName); cmdErr == nil {
		return libPath, nil
	}
	return "", fmt.Errorf("cannot found valid driver lib, fromEnv: %v, fromLdCmd: %v", envErr, cmdErr)
}
