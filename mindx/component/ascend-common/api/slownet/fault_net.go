// Copyright (c) Huawei Technologies Co., Ltd. 2025-2025. All rights reserved.

// Package slownet for net fault detect common
package slownet

import (
	"fmt"
	"os"
	"path/filepath"

	"ascend-common/common-utils/hwlog"
	"ascend-common/common-utils/utils"
)

const (
	rasNetRootPathKey = "RAS_NET_ROOT_PATH"
	netFaultSubPath   = "cluster"
	detectConf        = "cathelper.conf"
)

// GetRasNetRootPath get ras net fault detect root path from env
func GetRasNetRootPath() (string, error) {
	rootPath := os.Getenv(rasNetRootPathKey)
	if len(rootPath) == 0 {
		return "", fmt.Errorf("env %s not exists, please config it before starting", rasNetRootPathKey)
	}
	if !utils.IsDir(rootPath) {
		return "", fmt.Errorf("env %s=%s, which is not dir", rasNetRootPathKey, rootPath)
	}
	safeRootPath, err := utils.CheckPath(rootPath)
	if err != nil {
		return "", fmt.Errorf("env %s=%s, which is invalid, err: %v", rasNetRootPathKey, rootPath, err)
	}
	return safeRootPath, nil
}

// GetPingListFilePath get ping list task info file for ping mesh
func GetPingListFilePath(superPodId, serverIndex string) (string, error) {
	rootPath, err := GetRasNetRootPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(rootPath, netFaultSubPath, fmt.Sprintf("super-pod-%s", superPodId),
		fmt.Sprintf("ping_list_%s.json", serverIndex)), nil
}

// GetSuperPodInfoFilePath get super pod info file path
func GetSuperPodInfoFilePath(superPodID, superPodPrefix string) (string, error) {
	rootPath, err := GetRasNetRootPath()
	if err != nil {
		hwlog.RunLog.Errorf("get ras net root path failed, err : %v", err)
		return "", err
	}
	superPodPathName := fmt.Sprintf("%s-%s", superPodPrefix, superPodID)
	fileName := fmt.Sprintf("%s.json", superPodPathName)
	filePath := filepath.Join(rootPath, netFaultSubPath, superPodPathName, fileName)
	if _, errInfo := utils.CheckPath(filePath); errInfo != nil {
		hwlog.RunLog.Errorf("file path is invalid, err: %v", errInfo)
		return "", errInfo
	}
	return filePath, nil
}

// GetConfigPathForDetect the config path for network fault detect so
func GetConfigPathForDetect(superPodId string) (string, error) {
	rasNetRootPath, err := GetRasNetRootPath()
	if err != nil {
		hwlog.RunLog.Errorf("get ras net root path failed, err: %v", err)
		return "", err
	}
	confPath := filepath.Join(rasNetRootPath, netFaultSubPath, fmt.Sprintf("super-pod-%s", superPodId), detectConf)
	if _, errInfo := utils.CheckPath(confPath); errInfo != nil {
		hwlog.RunLog.Errorf("file path is invalid, err: %v", errInfo)
		return "", errInfo
	}
	return confPath, nil
}
