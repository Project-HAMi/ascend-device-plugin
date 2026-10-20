package server

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
)

// fileSHA256 calculates the SHA256 checksum of the specified file
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
	}()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Install by rename so an existing mapped library is never truncated in place.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()
	info, err := srcFile.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source %s is not a regular file", src)
	}
	staged, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(staged.Name()) }()
	defer func() { _ = staged.Close() }()
	if _, err := io.Copy(staged, srcFile); err != nil {
		return err
	}
	if err := staged.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := staged.Sync(); err != nil {
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	return os.Rename(staged.Name(), dst)
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

	// 2. Install into the same host directory used by Allocate.
	targetDir := hostHookPath
	if !filepath.IsAbs(targetDir) {
		return fmt.Errorf("HOOK_PATH must be absolute, got %q", targetDir)
	}
	if err := os.MkdirAll(targetDir, 0775); err != nil {
		return fmt.Errorf("failed to create %s: %w", targetDir, err)
	}

	// Specify the in-container assets directory (can be overridden via environment variable, default follows standard DevicePlugin convention)
	assetsDir := os.Getenv("HAMI_VNPU_ASSETS_PATH")
	if assetsDir == "" {
		assetsDir = "/usr/local/hami-vnpu-core-assets"
	}

	// Validate the complete bundle before replacing either installed asset.
	names := []string{"libvnpu.so", "ld.so.preload"}
	expected := make(map[string]string, len(names))
	for _, name := range names {
		source := filepath.Join(assetsDir, name)
		info, err := os.Stat(source)
		if err != nil {
			return fmt.Errorf("stat HAMi asset %s: %w", source, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("HAMi asset %s is not a regular file", source)
		}
		expected[name], err = fileSHA256(source)
		if err != nil {
			return fmt.Errorf("hash HAMi asset %s: %w", source, err)
		}
	}
	for _, name := range names {
		source, destination := filepath.Join(assetsDir, name), filepath.Join(targetDir, name)
		installed, err := fileSHA256(destination)
		if err != nil || installed != expected[name] {
			if err := copyFile(source, destination); err != nil {
				return fmt.Errorf("install HAMi asset %s: %w", destination, err)
			}
		}
		installed, err = fileSHA256(destination)
		if err != nil || installed != expected[name] {
			return fmt.Errorf("HAMi asset %s does not match bundled SHA256 after install", destination)
		}
		klog.Infof("HAMi asset ready: %s SHA256=%s", destination, installed)
	}

	klog.Info("Host resource preparation completed successfully.")
	return nil
}

// prepareENPUHostResources installs missing ENPU assets without replacing existing files.
func prepareENPUHostResources() error {
	root := enpuHostPath("ENPU_CONFIG_ROOT", defaultENPUConfigRoot)
	if err := os.MkdirAll(root, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", root, err)
	}

	assetsDir := "/usr/local/enpu-runtime-assets"
	runtimeDir := "/usr/local/enpu/vcann-rt"
	if err := os.MkdirAll(filepath.Join(runtimeDir, "lib"), 0755); err != nil {
		return fmt.Errorf("failed to create ENPU runtime directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(runtimeDir, "tools"), 0755); err != nil {
		return fmt.Errorf("failed to create ENPU tools directory: %w", err)
	}
	assets := []struct {
		name string
		dst  string
		mode os.FileMode
	}{
		{"libvruntime.so", filepath.Join(runtimeDir, "lib", "libvruntime.so"), 0444},
		{"enpu-monitor", filepath.Join(runtimeDir, "tools", "enpu-monitor"), 0555},
		{"ld.so.preload", filepath.Join(runtimeDir, "ld.so.preload"), 0444},
	}
	for _, asset := range assets {
		src := filepath.Join(assetsDir, asset.name)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("ENPU runtime asset %s is missing from image assets: %w", asset.name, err)
		}
		if _, err := os.Stat(asset.dst); err == nil {
			srcSum, srcErr := fileSHA256(src)
			dstSum, dstErr := fileSHA256(asset.dst)
			if srcErr == nil && dstErr == nil && srcSum == dstSum {
				if err := os.Chmod(asset.dst, asset.mode); err != nil {
					return fmt.Errorf("chmod existing ENPU runtime asset %s: %w", asset.dst, err)
				}
				continue
			}
			return fmt.Errorf("ENPU runtime asset %s already exists with different contents", asset.dst)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat ENPU runtime asset %s: %w", asset.dst, err)
		}
		if err := copyFile(src, asset.dst); err != nil {
			return fmt.Errorf("copy ENPU runtime asset %s to %s: %w", src, asset.dst, err)
		}
		if err := os.Chmod(asset.dst, asset.mode); err != nil {
			return fmt.Errorf("chmod ENPU runtime asset %s: %w", asset.dst, err)
		}
	}
	return nil
}
