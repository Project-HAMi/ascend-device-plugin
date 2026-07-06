package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

// fileSHA256 calculates the SHA256 checksum of the specified file
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// A standard file copy implementation that preserves the original file permissions
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

// Automatically creates directories, sets permissions, and copies core files on the host
func prepareHostResources() error {
	klog.Info("Starting host resource preparation for HAMi vNPU core...")

	// 1. Create shared memory directory
	sharedRegionPath := "/usr/local/hami-shared-region"
	if err := os.MkdirAll(sharedRegionPath, 0777); err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("failed to create %s: %w", sharedRegionPath, err)
		}
	}
	if err := os.Chmod(sharedRegionPath, 0777); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", sharedRegionPath, err)
	}
	klog.Infof("Successfully prepared directory: %s", sharedRegionPath)

	// 2. Prepare /usr/local/hami-vnpu-core/ directory
	targetDir := "/usr/local/hami-vnpu-core"
	if err := os.MkdirAll(targetDir, 0775); err != nil {
		return fmt.Errorf("failed to create %s: %w", targetDir, err)
	}

	// Specify the in-container assets directory (can be overridden via environment variable, default follows standard DevicePlugin convention)
	assetsDir := os.Getenv("HAMI_VNPU_ASSETS_PATH")
	if assetsDir == "" {
		assetsDir = "/usr/local/hami-vnpu-core-assets"
	}

	filesToCopy := map[string]string{
		"libvnpu.so":    filepath.Join(targetDir, "libvnpu.so"),
		"ld.so.preload": filepath.Join(targetDir, "ld.so.preload"),
	}

	for srcName, destPath := range filesToCopy {
		srcPath := filepath.Join(assetsDir, srcName)

		// File already exists, skip if content is consistent
		if _, err := os.Stat(destPath); err == nil {
			srcSum, err1 := fileSHA256(srcPath)
			dstSum, err2 := fileSHA256(destPath)

			if err1 == nil && err2 == nil && srcSum == dstSum {
				klog.Infof("✓ %s already up-to-date, skipping", destPath)
				continue
			}
		}

		if err := copyFile(srcPath, destPath); err != nil {
			if strings.Contains(err.Error(), "text file busy") {
				klog.Warningf("⚠ %s is in use by running process, keeping existing version (safe)", destPath)
				continue
			}
			return fmt.Errorf("failed to copy %s: %w", destPath, err)
		}
		klog.Infof("✓ Copied %s -> %s", srcPath, destPath)
	}

	klog.Info("Host resource preparation completed successfully.")
	return nil
}
