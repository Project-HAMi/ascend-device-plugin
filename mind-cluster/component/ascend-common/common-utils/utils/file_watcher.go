/* Copyright(C) 2025. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package utils offer utils for file watcher
package utils

import (
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher struct file watcher
type FileWatcher struct {
	watcher *fsnotify.Watcher
}

// NewFileWatcher new FileWatcher
func NewFileWatcher() (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &FileWatcher{watcher: watcher}, nil
}

// WatchFile add file to watch
func (fw *FileWatcher) WatchFile(filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		return err
	}
	if _, err := PathStringChecker(filePath); err != nil {
		return err
	}
	return fw.watcher.Add(filePath)
}

// Events get event channel
func (fw *FileWatcher) Events() chan fsnotify.Event {
	if fw == nil || fw.watcher == nil {
		return nil
	}
	return fw.watcher.Events
}

// Errors get error channel
func (fw *FileWatcher) Errors() chan error {
	if fw == nil || fw.watcher == nil {
		return nil
	}
	return fw.watcher.Errors
}

// Close to close the file watcher
func (fw *FileWatcher) Close() error {
	if fw == nil || fw.watcher == nil {
		return nil
	}
	return fw.watcher.Close()
}

// GetFileWatcherChan get eventCh and errCh for file watcher
func GetFileWatcherChan(filePath string) (*FileWatcher, error) {
	watcher, err := NewFileWatcher()
	if err != nil {
		return nil, fmt.Errorf("new file watcher failed, error: %v", err)
	}
	if err = watcher.WatchFile(filePath); err != nil {
		return nil, fmt.Errorf("watch file <%s> failed, error: %v", filePath, err)
	}
	fmt.Printf("watching file <%s>...\n", filePath)
	return watcher, nil
}
