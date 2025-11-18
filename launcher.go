package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
)

// =============================
// 全局配置变量
// =============================
var (
	appName         = "AMMDS"
	appExecutable   = "ammds.exe"
	appIconFile     = "icon.ico"
	maxRestarts     = 5
	restartCooldown = 3 * time.Second

	// 环境变量
	defaultEnv = map[string]string{
		"AMMDS_SERVER_PORT":     "0",
		"ADMIN_USER":            "ammds",
		"AMMDS_SYSTEM_MODE":     "full",
		"AMMDS_NETWORK_TIMEOUT": "60",
	}

	// 进程控制
	cmdLock    sync.Mutex
	appCmd     *exec.Cmd
	shouldRun  = true
	shouldLock sync.Mutex

	// 运行时状态
	runningLock sync.Mutex
	isRunning   = false

	// 控制通道（用于托盘命令 -> 守护循环）
	controlCh = make(chan string, 1)
	statusCh  = make(chan string, 5)
)

// =============================
// 工具函数区域
// =============================
func getAppDir() string {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal("无法获取程序目录: ", err)
	}
	return filepath.Dir(exe)
}

func getFreePort() int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 30000
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func openBrowser(url string) {
	// windows default
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func loadIconBytes() []byte {
	iconPath := filepath.Join(getAppDir(), appIconFile)
	b, err := os.ReadFile(iconPath)
	if err != nil {
		log.Printf("无法读取图标 %s: %v", iconPath, err)
		return nil
	}
	return b
}

func setRunningState(r bool) {
	runningLock.Lock()
	isRunning = r
	runningLock.Unlock()
}

func getRunningState() bool {
	runningLock.Lock()
	defer runningLock.Unlock()
	return isRunning
}

func setShouldRun(v bool) {
	shouldLock.Lock()
	shouldRun = v
	shouldLock.Unlock()
}

func getShouldRun() bool {
	shouldLock.Lock()
	defer shouldLock.Unlock()
	return shouldRun
}

// =============================
// 启动后端
// =============================
func startBackend(port int) error {
	cmdLock.Lock()
	defer cmdLock.Unlock()

	// If already running, do nothing
	if appCmd != nil && appCmd.Process != nil {
		if getRunningState() {
			return nil
		}
	}

	appCmd = exec.Command(filepath.Join(getAppDir(), appExecutable))
	appCmd.Dir = getAppDir()
	appCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	env := os.Environ()
	localEnv := make(map[string]string)

	for k, v := range defaultEnv {
		localEnv[k] = v
	}

	localEnv["AMMDS_SERVER_PORT"] = fmt.Sprintf("%d", port)

	// 系统环境覆盖默认
	for k := range localEnv {
		if sysVal, ok := os.LookupEnv(k); ok {
			localEnv[k] = sysVal
		}
	}

	for k, v := range localEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	appCmd.Env = env

	// 启动
	if err := appCmd.Start(); err != nil {
		appCmd = nil
		return err
	}
	setRunningState(true)
	statusCh <- fmt.Sprintf("started pid=%d", appCmd.Process.Pid)
	return nil
}

// 停止后端（用户主动停止）
func stopBackend() {
	cmdLock.Lock()
	defer cmdLock.Unlock()

	if appCmd != nil && appCmd.Process != nil {
		// 在 Windows 上使用 Kill
		_ = appCmd.Process.Kill()
		_, _ = appCmd.Process.Wait()
		statusCh <- "killed"
	}
	appCmd = nil
	setRunningState(false)
}

// =============================
// 守护循环（单端口，不重复生成）
// =============================
func daemonLoop(port int) {
	restarts := 0

	for {
		select {
		case cmd := <-controlCh:
			switch cmd {
			case "start":
				setShouldRun(true)
			case "stop":
				setShouldRun(false)
				stopBackend()
			case "restart":
				stopBackend()
				setShouldRun(true)
			case "quit":
				setShouldRun(false)
				stopBackend()
				statusCh <- "quit"
				return
			}
		default:
		}

		if !getShouldRun() {
			cmd := <-controlCh
			switch cmd {
			case "start":
				setShouldRun(true)
			case "quit":
				statusCh <- "quit"
				return
			case "restart":
				setShouldRun(true)
			}
		}

		// 启动 backend
		err := startBackend(port)
		if err != nil {
			log.Printf("启动失败: %v", err)
			statusCh <- fmt.Sprintf("start_error:%v", err)
			restarts++
			if restarts > maxRestarts {
				log.Printf("重启次数超过限制（%d 次），退出守护。", maxRestarts)
				return
			}
			time.Sleep(restartCooldown)
			continue
		}

		waitCh := make(chan error, 1)
		go func(cmd *exec.Cmd) {
			waitCh <- cmd.Wait()
		}(appCmd)

		select {
		case err := <-waitCh:
			setRunningState(false)
			if err != nil {
				log.Printf("程序退出（异常）: %v", err)
				statusCh <- fmt.Sprintf("exited_error:%v", err)
			} else {
				log.Printf("程序退出（正常）")
				statusCh <- "exited_ok"
			}

			if getShouldRun() {
				restarts++
				if restarts > maxRestarts {
					log.Printf("重启次数超过限制（%d 次），退出守护。", maxRestarts)
					return
				}
				log.Printf("%d 秒后尝试重启（第 %d/%d 次）...", restartCooldown/time.Second, restarts, maxRestarts)
				time.Sleep(restartCooldown)
				continue
			} else {
				continue
			}
		case cmd := <-controlCh:
			switch cmd {
			case "stop":
				setShouldRun(false)
				stopBackend()
				continue
			case "restart":
				stopBackend()
				continue
			case "quit":
				setShouldRun(false)
				stopBackend()
				statusCh <- "quit"
				return
			case "start":
			}
		}
	}
}

// =============================
// 主入口 + 托盘
// =============================
func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	if icon := loadIconBytes(); icon != nil {
		systray.SetIcon(icon)
	}
	systray.SetTitle("AMMDS Launcher")
	systray.SetTooltip("AMMDS 后端服务守护程序")

	mStart := systray.AddMenuItem("启动", "启动后端服务")
	mStop := systray.AddMenuItem("停止", "停止后端服务")
	mRestart := systray.AddMenuItem("重启", "重启后端服务")
	systray.AddSeparator()
	mOpen := systray.AddMenuItem("打开面板", "打开 Web UI")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出程序")

	port := getFreePort()
	log.Printf("启动 %s，端口: %d", appName, port)
	openBrowser(fmt.Sprintf("http://localhost:%d", port))

	go daemonLoop(port)

	go func() {
		for {
			select {
			case <-mStart.ClickedCh:
				controlCh <- "start"
			case <-mStop.ClickedCh:
				controlCh <- "stop"
			case <-mRestart.ClickedCh:
				controlCh <- "restart"
			case <-mOpen.ClickedCh:
				openBrowser(fmt.Sprintf("http://localhost:%d", port))
			case <-mQuit.ClickedCh:
				controlCh <- "quit"
				// 退出 systray
				systray.Quit()
				return
			}
		}
	}()

	go func() {
		for {
			time.Sleep(800 * time.Millisecond)
			r := getRunningState()
			if r {
				mStart.Disable()
				mStop.Enable()
			} else {
				mStart.Enable()
				mStop.Disable()
			}
		}
	}()

	go func() {
		for s := range statusCh {
			log.Println("[STATUS]", s)
		}
	}()
}

func onExit() {
	controlCh <- "quit"
	stopBackend()
}
