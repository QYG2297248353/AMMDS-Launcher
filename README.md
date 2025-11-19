# AMMDS Launcher

AMMDS Launcher 是一个 Windows 系统托盘应用程序，用于启动和管理 AMMDS 后端服务。该程序会在系统托盘中运行，提供启动、停止、重启 AMMDS 服务以及打开 Web 界面等功能。

## 功能特性

- 系统托盘集成，不显示命令行窗口
- 自动监听可用端口并启动 AMMDS 服务
- 提供启动、停止、重启服务功能
- 一键打开 Web 管理界面
- 服务崩溃后自动重启（最多5次）
- 开机自启支持

## 开发环境要求

### 基础环境

- Go 1.25+
- Windows 系统（由于使用了 Windows 特定的系统调用）
- CGO 启用（默认启用，但确保未被禁用）

### C 编译器要求

由于项目使用了 [github.com/getlantern/systray](https://github.com/getlantern/systray) 库，该库依赖 CGO，因此需要安装支持 CGO 的 C 编译器：

#### 推荐方案：安装 MinGW-w64

1. 从 [MinGW-w64 官网](http://mingw-w64.org/doku.php/download) 下载安装程序
2. 运行安装程序，推荐选择：
   - Version: 最新版本
   - Architecture: x86_64
   - Threads: win32
   - Exception: seh
3. 将 MinGW-w64 的 bin 目录（例如：`C:\mingw64\bin`）添加到系统 PATH 环境变量中
4. 验证安装：
   ```bash
   gcc --version
   ```

#### 替代方案：安装 TDM-GCC

1. 从 [TDM-GCC 官网](https://jmeubank.github.io/tdm-gcc/) 下载安装程序
2. 运行安装程序并完成安装
3. 确保安装路径已添加到系统 PATH 环境变量中

#### 验证 CGO 支持

在项目目录中执行以下命令验证 CGO 是否正常工作：

```bash
go env CGO_ENABLED
# 应输出 1

gcc --version
# 应显示 GCC 版本信息
```

### Go 依赖包

项目使用 Go Modules 管理依赖，主要依赖包括：

- `github.com/getlantern/systray`: 用于创建系统托盘图标和菜单

首次构建时，Go 会自动下载所需的依赖包。也可以手动执行以下命令来下载和安装依赖：

```bash
# 下载并安装所有依赖
go mod download

# 或者在项目根目录执行以下命令确保依赖完整
go mod tidy
```

## 构建说明

### 方法一：使用 PowerShell 脚本构建（推荐）

项目提供了一个 PowerShell 构建脚本 [build.ps1](build.ps1)，可以自动完成构建和打包过程：

```powershell
# 基本构建
.\build.ps1

# 指定版本号构建
.\build.ps1 -Version "1.6.33"

# 仅构建可执行文件，不创建安装包
.\build.ps1 -NoInstaller

# 仅创建安装包，不重新构建可执行文件
.\build.ps1 -NoBuild
```

### 方法二：手动构建命令

使用以下命令构建无控制台窗口的 Windows 可行文件：

```bash
# 确保依赖完整
go mod tidy

# 构建
go build -ldflags "-H windowsgui" -o AMMDS-Launcher.exe
```

参数说明：
- `go mod tidy`: 整理并下载缺失的依赖
- `-ldflags "-H windowsgui"`: 创建 Windows GUI 应用程序，不显示命令行窗口
- `-o AMMDS-Launcher.exe`: 指定输出文件名

### 依赖项

项目使用以下主要依赖：

- `github.com/getlantern/systray`: 用于创建系统托盘图标和菜单

所有依赖项可通过 `go.mod` 文件管理。

## 安装打包

项目包含 Inno Setup 安装脚本 [installer.iss](installer.iss)，可用于创建 Windows 安装包。

### 使用 PowerShell 脚本打包（推荐）

PowerShell 构建脚本 [build.ps1](build.ps1) 会自动查找系统中的 Inno Setup 编译器并创建安装包。

### 手动使用 Inno Setup 打包

1. 安装 Inno Setup 6.0 或更高版本
2. 打开 [installer.iss](installer.iss) 脚本文件
3. 选择 `Build` → `Compile` 编译安装包
4. 生成的安装包位于输出目录中

安装包包含以下组件：
- AMMDS-Launcher.exe: 启动器主程序
- ammds.exe: AMMDS 后端服务程序
- icon.ico/logo.ico: 图标文件

安装选项：
- 创建桌面图标
- 创建开始菜单项
- 设置开机自动启动
- 安装完成后立即启动 AMMDS

## 使用方法

1. 运行 AMMDS-Launcher.exe
2. 程序将在系统托盘运行（右下角通知区域）
3. 右键点击托盘图标可访问以下功能：
   - 启动：启动 AMMDS 后端服务
   - 停止：停止 AMMDS 后端服务
   - 重启：重启 AMMDS 后端服务
   - 打开面板：在浏览器中打开 AMMDS Web 界面
   - 退出：完全退出 Launcher 程序

程序首次启动时会自动：
1. 寻找可用端口
2. 启动 AMMDS 后端服务
3. 在默认浏览器中打开 Web 界面

## 配置说明

程序会自动设置以下环境变量：

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| AMMDS_SERVER_PORT | 自动分配 | 服务监听端口 |
| ADMIN_USER | ammds | 管理员用户名 |
| AMMDS_SYSTEM_MODE | full | 系统模式 |
| AMMDS_NETWORK_TIMEOUT | 60 | 网络超时时间(秒) |

这些默认值可以通过系统环境变量覆盖。

工作目录位于 `%LOCALAPPDATA%\AMMDS`。

## 故障排除

### 服务无法启动
- 确保 ammds.exe 文件存在于同一目录下
- 检查 Windows 事件日志获取详细错误信息
- 尝试手动运行 ammds.exe 查看是否有错误输出

### 程序崩溃
- Launcher 会自动重启崩溃的服务（最多5次）
- 查看日志文件获取更多信息

### 托盘图标不显示
- 确认 Windows Explorer 正常运行
- 尝试重启 Windows Explorer 进程

### Go 依赖问题
- 确保网络连接正常，以便下载依赖
- 执行 `go mod tidy` 命令同步依赖
- 清理模块缓存后重试：`go clean -modcache` 然后重新构建

## 许可证

请根据实际项目许可证情况填写此部分。