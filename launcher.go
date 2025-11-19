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

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/getlantern/systray"
)

var (
	appExecutable   = "ammds.exe"
	appIconFile     = "icon.ico"
	maxRestarts     = 5
	restartCooldown = 3 * time.Second
	restartCount    = 0

	defaultEnv = map[string]string{
		"AMMDS_SERVER_PORT":     "0",
		"ADMIN_USER":            "ammds",
		"AMMDS_SYSTEM_MODE":     "full",
		"AMMDS_NETWORK_TIMEOUT": "60",
	}

	cmdLock    sync.Mutex
	appCmd     *exec.Cmd
	shouldRun  = true
	shouldLock sync.Mutex

	runningLock sync.Mutex
	isRunning   = false

	userStopped = false
	userLock    sync.Mutex

	controlCh = make(chan string, 1)
	statusCh  = make(chan string, 5)

	logFile *os.File

	singletonLockFile *os.File
)

// =============================
// 工具函数
// =============================
func getAppDir() string {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Dir(exe)
}

func getWorkDir() string {
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "AMMDS")
	os.MkdirAll(dir, 0755)
	return dir
}

func getLogDir() string {
	dir := filepath.Join(getWorkDir(), "logs")
	os.MkdirAll(dir, 0755)
	return dir
}

func acquireSingletonLock() bool {
	lockFilePath := filepath.Join(getWorkDir(), "ammds_launcher.lock")
	file, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return false
	}

	if err := lockFile(int(file.Fd())); err != nil {
		file.Close()
		return false
	}

	singletonLockFile = file
	return true
}

func lockFile(fd int) error {
	return windows.LockFileEx(windows.Handle(fd), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &windows.Overlapped{})
}

func releaseSingletonLock() {
	if singletonLockFile != nil {
		singletonLockFile.Close()
		lockFilePath := filepath.Join(getWorkDir(), "ammds_launcher.lock")
		os.Remove(lockFilePath)
		singletonLockFile = nil
	}
}

func initLogger() {
	logPath := filepath.Join(getLogDir(), "launcher.log")

	if _, err := os.Stat(logPath); err == nil {
		f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			os.Remove(logPath)
			f, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		}
		if err == nil {
			f.Close()
		}
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(f)
		logFile = f
	}
}

func openFolder(path string) {
	_ = exec.Command("explorer", path).Start()
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
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func loadIconBytes() []byte {
	path := filepath.Join(getAppDir(), appIconFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return b
}

func setRunningState(v bool) {
	runningLock.Lock()
	isRunning = v
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

func setUserStopped(v bool) {
	userLock.Lock()
	userStopped = v
	userLock.Unlock()
}

func getUserStopped() bool {
	userLock.Lock()
	defer userLock.Unlock()
	return userStopped
}

// =============================
// 后端控制
// =============================
func startBackend(port int) error {
	cmdLock.Lock()
	defer cmdLock.Unlock()

	if appCmd != nil && appCmd.Process != nil && getRunningState() {
		log.Println("Backend is already running")
		return nil
	}

	log.Printf("Starting backend on port %d", port)
	appCmd = exec.Command(filepath.Join(getAppDir(), appExecutable))
	appCmd.Dir = getWorkDir()
	appCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	appCmd.Stdout = logFile
	appCmd.Stderr = logFile

	env := os.Environ()
	for k, v := range defaultEnv {
		if sysVal, ok := os.LookupEnv(k); ok {
			v = sysVal
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	env = append(env, fmt.Sprintf("AMMDS_SERVER_PORT=%d", port))

	appCmd.Env = env

	if err := appCmd.Start(); err != nil {
		log.Printf("Failed to start backend: %v", err)
		appCmd = nil
		return err
	}

	log.Printf("Backend started with PID: %d", appCmd.Process.Pid)
	setRunningState(true)
	statusCh <- fmt.Sprintf("started pid=%d", appCmd.Process.Pid)
	return nil
}

func stopBackend() {
	cmdLock.Lock()
	defer cmdLock.Unlock()

	if appCmd != nil && appCmd.Process != nil {
		log.Printf("Stopping backend with PID: %d", appCmd.Process.Pid)
		err := appCmd.Process.Signal(os.Interrupt)
		if err != nil {
			log.Printf("Failed to send interrupt signal: %v", err)
			_ = appCmd.Process.Kill()
		}

		done := make(chan error, 1)
		go func() {
			done <- appCmd.Wait()
		}()

		select {
		case <-done:
			log.Println("Backend stopped gracefully")
			statusCh <- "backend stopped gracefully"
		case <-time.After(5 * time.Second):
			log.Println("Backend stop timeout, killing forcefully")
			_ = appCmd.Process.Kill()
			appCmd.Wait()
			statusCh <- "backend killed forcefully"
		}
	} else {
		log.Println("Backend is not running")
	}

	appCmd = nil
	setRunningState(false)
}

// =============================
// 守护
// =============================
func daemonLoop(port int) {
	restartCount = 0

	for {
		select {
		case c := <-controlCh:
			log.Printf("Received control command: %s", c)
			switch c {
			case "start":
				setShouldRun(true)
				setUserStopped(false)
				restartCount = 0
			case "stop":
				setShouldRun(false)
				setUserStopped(true)
				stopBackend()
				restartCount = 0
			case "restart":
				stopBackend()
				setShouldRun(true)
				setUserStopped(false)
				restartCount = 0
			case "quit":
				setShouldRun(false)
				setUserStopped(true)
				stopBackend()
				statusCh <- "quit"
				return
			}
		default:
		}

		if !getShouldRun() || getUserStopped() {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		err := startBackend(port)
		if err != nil {
			statusCh <- fmt.Sprintf("failed to start: %v", err)
			restartCount++

			if restartCount >= maxRestarts {
				statusCh <- fmt.Sprintf("exceeded max restarts (%d), giving up", maxRestarts)
				setShouldRun(false)
				continue
			}

			time.Sleep(restartCooldown)
			continue
		}

		waitCh := make(chan error, 1)
		go func(cmd *exec.Cmd) { waitCh <- cmd.Wait() }(appCmd)

		select {
		case err := <-waitCh:
			setRunningState(false)
			statusCh <- fmt.Sprintf("process exited with error: %v", err)

			if getShouldRun() && !getUserStopped() {
				restartCount++

				if restartCount >= maxRestarts {
					statusCh <- fmt.Sprintf("exceeded max restarts (%d), giving up", maxRestarts)
					setShouldRun(false)
					continue
				}

				time.Sleep(restartCooldown)
			}
		case c := <-controlCh:
			if c == "stop" || c == "restart" || c == "quit" {
				if c == "stop" {
					setUserStopped(true)
				}
				stopBackend()
				restartCount = 0

				if c == "quit" {
					return
				}
			}
		}
	}
}

// =============================
// 卸载
// =============================
func handleUninstall() {
	log.Println("Received --uninstall: stopping backend...")

	if isAutoStartEnabled() {
		log.Println("Auto-start is enabled, disabling it...")
		_ = setAutoStart(false)
	}

	setShouldRun(false)

	stopBackend()

	killAllAMMDSProcesses()

	select {
	case controlCh <- "quit":
		log.Println("Sent quit signal to systray")
	case <-time.After(1 * time.Second):
		log.Println("Timeout sending quit signal, proceeding anyway")
	}

	time.Sleep(3 * time.Second)

	if logFile != nil {
		logFile.Close()
	}

	releaseSingletonLock()

	os.Exit(0)
}

func killAllAMMDSProcesses() {
	cmd := exec.Command("taskkill", "/F", "/IM", "ammds.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()

	cmd = exec.Command("taskkill", "/F", "/IM", "AMMDS-Launcher.exe", "/FI", "PID ne "+fmt.Sprintf("%d", os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()

	log.Println("Attempted to kill all AMMDS related processes")
}

// =============================
// 主入口
// =============================
func main() {
	initLogger()

	for _, arg := range os.Args {
		if arg == "--uninstall" {
			handleUninstall()
			return
		}
	}

	if !acquireSingletonLock() {
		log.Println("AMMDS Launcher is already running, exiting")
		os.Exit(1)
	}

	log.Println("Starting AMMDS Launcher...")

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
	mData := systray.AddMenuItem("打开数据目录", "打开数据存储目录")

	systray.AddSeparator()
	mOpen := systray.AddMenuItem("打开面板", "打开 Web UI")

	autoStartEnabled := isAutoStartEnabled()
	var mAutoStart *systray.MenuItem
	if autoStartEnabled {
		mAutoStart = systray.AddMenuItem("禁用自动启动", "开机时不要自动启动")
	} else {
		mAutoStart = systray.AddMenuItem("启用自动启动", "开机时自动启动")
	}

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出程序")

	port := getFreePort()

	go func() {
		time.Sleep(3 * time.Second)
		openBrowser(fmt.Sprintf("http://localhost:%d", port))
	}()

	go daemonLoop(port)

	go func() {
		for {
			select {
			case <-mStart.ClickedCh:
				select {
				case controlCh <- "start":
				default:
					log.Println("Control channel is full, dropping start command")
				}
			case <-mStop.ClickedCh:
				select {
				case controlCh <- "stop":
				default:
					log.Println("Control channel is full, dropping stop command")
				}
			case <-mRestart.ClickedCh:
				select {
				case controlCh <- "restart":
				default:
					log.Println("Control channel is full, dropping restart command")
				}
			case <-mData.ClickedCh:
				openFolder(getWorkDir())
			case <-mOpen.ClickedCh:
				openBrowser(fmt.Sprintf("http://localhost:%d", port))
			case <-mAutoStart.ClickedCh:
				currentStatus := isAutoStartEnabled()
				if currentStatus {
					err := setAutoStart(false)
					if err != nil {
						log.Printf("Failed to disable auto start: %v", err)
					} else {
						mAutoStart.SetTitle("启用自动启动")
						mAutoStart.SetTooltip("开机时自动启动")
					}
				} else {
					err := setAutoStart(true)
					if err != nil {
						log.Printf("Failed to enable auto start: %v", err)
					} else {
						mAutoStart.SetTitle("禁用自动启动")
						mAutoStart.SetTooltip("开机时不要自动启动")
					}
				}
			case <-mQuit.ClickedCh:
				select {
				case controlCh <- "quit":
				default:
					log.Println("Control channel is full, proceeding with quit anyway")
				}
				time.Sleep(500 * time.Millisecond)
				systray.Quit()
				return
			}
		}
	}()

	go func() {
		for {
			time.Sleep(800 * time.Millisecond)
			if getRunningState() {
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
	log.Println("AMMDS Launcher is exiting...")
	select {
	case controlCh <- "quit":
		log.Println("Sent quit signal in onExit")
	case <-time.After(500 * time.Millisecond):
		log.Println("Timeout sending quit signal in onExit")
	}
	stopBackend()
	if logFile != nil {
		logFile.Close()
	}
	releaseSingletonLock()
	log.Println("AMMDS Launcher has exited cleanly")
}

func isAutoStartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.READ)
	if err != nil {
		return false
	}
	defer k.Close()

	_, _, err = k.GetStringValue("AMMDS Launcher")
	return err == nil
}

func setAutoStart(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enabled {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		return k.SetStringValue("AMMDS Launcher", executable)
	} else {
		return k.DeleteValue("AMMDS Launcher")
	}
}
