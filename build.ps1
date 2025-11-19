# AMMDS Launcher 构建脚本
# 用于自动化构建和打包 AMMDS Launcher

param(
    [Parameter(Mandatory=$false)]
    [String]$Version = "1.0.0",
    
    [Parameter(Mandatory=$false)]
    [Switch]$NoBuild,

    [Parameter(Mandatory=$false)]
    [Switch]$NoInstaller
)

Write-Host "========================================" -ForegroundColor Green
Write-Host "AMMDS Launcher 构建脚本" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

# 检查 Go 是否安装
$goVersion = Get-Command go -ErrorAction SilentlyContinue
if (-not $goVersion) {
    Write-Error "错误: 未找到 Go 编译器。请先安装 Go 1.25 或更高版本。"
    exit 1
}

# 获取 Go 版本
$goVer = go version
Write-Host "Go 版本: $goVer" -ForegroundColor Cyan

# 检查 CGO 是否启用
$cgoEnabled = $(go env CGO_ENABLED)
if ($cgoEnabled -ne "1") {
    Write-Warning "警告: CGO 未启用，可能会导致构建失败。"
}

# 检查 GCC 是否可用
$gccVersion = Get-Command gcc -ErrorAction SilentlyContinue
if (-not $gccVersion) {
    Write-Warning "警告: 未找到 GCC 编译器。如果项目依赖 CGO，构建可能会失败。"
} else {
    $gccVer = gcc --version
    Write-Host "GCC 版本: $($gccVer[0])" -ForegroundColor Cyan
}

# 创建输出目录
$outputDir = "dist"
if (!(Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

# 构建项目
if (-not $NoBuild) {
    Write-Host "`n正在构建 AMMDS Launcher..." -ForegroundColor Yellow
    
    # 清理之前的构建
    go clean
    
    # 构建 Windows GUI 应用
    $buildArgs = @(
        "build",
        "-ldflags", '"-H windowsgui"',
        "-o", "$outputDir\AMMDS-Launcher.exe"
    )
    
    $result = Start-Process -FilePath "go" -ArgumentList $buildArgs -NoNewWindow -Wait -PassThru -WorkingDirectory $(Get-Location)
    
    if ($result.ExitCode -eq 0) {
        Write-Host "构建成功!" -ForegroundColor Green
        
        # 显示生成的文件信息
        $exeInfo = Get-Item "$outputDir\AMMDS-Launcher.exe"
        Write-Host "输出文件: $($exeInfo.FullName)" -ForegroundColor Cyan
        Write-Host "文件大小: $([math]::Round($exeInfo.Length / 1KB, 2)) KB" -ForegroundColor Cyan
        
        # 复制必要的依赖文件到 dist 目录
        $requiredFiles = @("ammds.exe", "icon.ico", "icon.png")
        foreach ($file in $requiredFiles) {
            if (Test-Path $file) {
                Copy-Item $file $outputDir -Force
                Write-Host "已复制文件: $file" -ForegroundColor Cyan
            } else {
                Write-Warning "缺少必要文件: $file （安装包可能无法正常工作）"
            }
        }
    } else {
        Write-Error "构建失败! ExitCode: $($result.ExitCode)"
        exit $result.ExitCode
    }
} else {
    Write-Host "`n跳过构建步骤..." -ForegroundColor Yellow
}

# 创建安装包
if (-not $NoInstaller) {
    Write-Host "`n正在检查 Inno Setup 编译器..." -ForegroundColor Yellow
    
    # 查找 Inno Setup 编译器
    $isccPaths = @(
        "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
        "${env:ProgramFiles}\Inno Setup 6\ISCC.exe",
        "${env:ProgramFiles(x86)}\Inno Setup 5\ISCC.exe",
        "${env:ProgramFiles}\Inno Setup 5\ISCC.exe"
    )
    
    $isccPath = $null
    foreach ($path in $isccPaths) {
        if (Test-Path $path) {
            $isccPath = $path
            break
        }
    }
    
    if ($isccPath) {
        Write-Host "找到 Inno Setup 编译器: $isccPath" -ForegroundColor Cyan
        
        # 更新版本号到 iss 脚本
        $issFile = "installer.iss"
        if (Test-Path $issFile) {
            $content = Get-Content $issFile
            $content = $content -replace 'AppVersion=.*', "AppVersion=$Version"
            Set-Content $issFile $content
            
            Write-Host "已更新安装脚本版本号为: $Version" -ForegroundColor Cyan
            
            # 编译安装包
            Write-Host "`n正在编译安装包..." -ForegroundColor Yellow
            
            $issArgs = @(
                "$issFile"
            )
            
            # 确保在正确的目录运行 Inno Setup
            $result = Start-Process -FilePath $isccPath -ArgumentList $issArgs -NoNewWindow -Wait -PassThru -WorkingDirectory $(Get-Location)
            
            if ($result.ExitCode -eq 0) {
                Write-Host "安装包编译成功!" -ForegroundColor Green
                
                # 查找生成的安装包
                $setupFiles = Get-ChildItem -Path . -Filter "*.exe" | Where-Object {$_.Name -like "*setup*"}
                if ($setupFiles) {
                    foreach ($setupFile in $setupFiles) {
                        $setupInfo = Get-Item $setupFile.FullName
                        Write-Host "安装包: $($setupInfo.FullName)" -ForegroundColor Cyan
                        Write-Host "文件大小: $([math]::Round($setupInfo.Length / 1KB, 2)) KB" -ForegroundColor Cyan
                    }
                } else {
                    Write-Warning "未找到生成的安装包文件"
                }
            } else {
                Write-Error "安装包编译失败! ExitCode: $($result.ExitCode)"
            }
        } else {
            Write-Warning "未找到安装脚本文件: $issFile"
        }
    } else {
        Write-Warning "未找到 Inno Setup 编译器。请安装 Inno Setup 5 或 6。"
        Write-Host "下载地址: http://www.jrsoftware.org/isinfo.php" -ForegroundColor Cyan
    }
} else {
    Write-Host "`n跳过安装包创建步骤..." -ForegroundColor Yellow
}

Write-Host "`n========================================" -ForegroundColor Green
Write-Host "构建脚本执行完毕" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green