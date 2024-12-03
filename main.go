package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <path-to-app> <output-dmg-path> <volume-name>")
		return
	}

	appPath := os.Args[1]
	dmgPath := os.Args[2]
	var volumeName string // 用户提供的挂载卷名称

	if len(os.Args) > 3 && os.Args[3] != "" {
		volumeName = os.Args[3]
	} else {
		volumeName = "My DMG"
	}

	// 检查 .app 文件是否存在
	if _, err := os.Stat(appPath); os.IsNotExist(err) {
		fmt.Printf("Error: .app file not found at %s\n", appPath)
		return
	}

	// 确保 .app 路径是绝对路径
	absAppPath, err := filepath.Abs(appPath)
	if err != nil {
		fmt.Printf("Error: Failed to resolve absolute path for %s\n", appPath)
		return
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "dmgtemp")
	if err != nil {
		fmt.Printf("Error: Failed to create temporary directory: %v\n", err)
		return
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tempDir) // 清理临时目录

	// 复制 .app 到临时目录
	appName := filepath.Base(absAppPath)
	destAppPath := filepath.Join(tempDir, appName)
	cmd := exec.Command("cp", "-R", absAppPath, destAppPath)
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error: Failed to copy .app file: %v\n", err)
		return
	}

	// 创建 /Applications 软链接
	applicationsLink := filepath.Join(tempDir, "Applications")
	err = os.Symlink("/Applications", applicationsLink)
	if err != nil {
		fmt.Printf("Error: Failed to create symlink: %v\n", err)
		return
	}

	// 创建 DMG 文件 (可写模式)
	fmt.Println("Creating writable DMG file...")
	tempDmgPath := dmgPath[:len(dmgPath)-len(filepath.Ext(dmgPath))] + "_temp.dmg"
	cmd = exec.Command("hdiutil", "create", "-volname", volumeName, "-srcfolder", tempDir, "-ov", "-format", "UDRW", tempDmgPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error: Failed to create writable DMG: %v\n", err)
		return
	}

	// 挂载 DMG 文件
	fmt.Println("Mounting DMG file...")
	mountPoint, err := os.MkdirTemp("", "dmgmount")
	if err != nil {
		fmt.Printf("Error: Failed to create mount point: %v\n", err)
		return
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(mountPoint)

	cmd = exec.Command("hdiutil", "attach", tempDmgPath, "-mountpoint", mountPoint, "-owners", "on")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error: Failed to mount DMG: %v\n", err)
		return
	}

	// 确保挂载点中的 /Applications 软链接正确创建
	destAppLink := filepath.Join(mountPoint, "Applications")
	if _, err := os.Lstat(destAppLink); !os.IsNotExist(err) {
		fmt.Println("Symlink already exists in DMG mount point. Skipping creation.")
	} else {
		err = os.Symlink("/Applications", destAppLink)
		if err != nil {
			fmt.Printf("Error: Failed to create symlink in DMG: %v\n", err)
			// 卸载挂载点后退出
			_ = exec.Command("hdiutil", "detach", mountPoint).Run()
			return
		}
	}

	// 卸载 DMG
	fmt.Println("Unmounting DMG file...")
	cmd = exec.Command("hdiutil", "detach", mountPoint)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error: Failed to unmount DMG: %v\n", err)
		return
	}

	// 将 DMG 转换为压缩格式
	fmt.Println("Converting DMG to compressed format...")
	cmd = exec.Command("hdiutil", "convert", tempDmgPath, "-format", "UDZO", "-o", dmgPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error: Failed to convert DMG to compressed format: %v\n", err)
		return
	}

	// 删除临时的可写 DMG 文件
	fmt.Println("Deleting temporary writable DMG file...")
	err = os.Remove(tempDmgPath)
	if err != nil {
		fmt.Printf("Error: Failed to delete temporary DMG: %v\n", err)
		return
	}

	fmt.Printf("DMG successfully created at %s with volume name: %s\n", dmgPath, volumeName)
}
