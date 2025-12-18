# AMMDS Launcher 构建脚本
# 用于自动化构建和打包 AMMDS Launcher

param(
    [Parameter(Mandatory=$false)]
    [String]$Version,
    
    [Parameter(Mandatory=$false)]
    [Switch]$NoBuild,

    [Parameter(Mandatory=$false)]
    [Switch]$NoInstaller
)

Write-Host "========================================" -ForegroundColor Green
Write-Host "AMMDS Launcher 构建脚本" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green

if (-not $Version) {
    $versionFile = "version"
    if (Test-Path $versionFile) {
        $Version = Get-Content $versionFile -Raw | ForEach-Object { $_.Trim() }
        if ($Version) {
            Write-Host "从文件读取版本号: $Version" -ForegroundColor Cyan
        } else {
            $Version = "1.0.0"
            Write-Host "使用默认版本号: $Version" -ForegroundColor Yellow
        }
    } else {
        $Version = "1.0.0"
        Write-Host "使用默认版本号: $Version" -ForegroundColor Yellow
    }
} else {
    Write-Host "使用指定版本号: $Version" -ForegroundColor Cyan
}

$goVersion = Get-Command go -ErrorAction SilentlyContinue
if (-not $goVersion) {
    Write-Error "错误: 未找到 Go 编译器。请先安装 Go 1.25 或更高版本。"
    exit 1
}

$goVer = go version
Write-Host "Go 版本: $goVer" -ForegroundColor Cyan

$cgoEnabled = $(go env CGO_ENABLED)
if ($cgoEnabled -ne "1") {
    Write-Warning "警告: CGO 未启用，可能会导致构建失败。"
}

$gccVersion = Get-Command gcc -ErrorAction SilentlyContinue
if (-not $gccVersion) {
    Write-Warning "警告: 未找到 GCC 编译器。如果项目依赖 CGO，构建可能会失败。"
} else {
    $gccVer = gcc --version
    Write-Host "GCC 版本: $($gccVer[0])" -ForegroundColor Cyan
}

$outputDir = "dist"
if (!(Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir | Out-Null
}

if (-not $NoBuild) {
    Write-Host "`n正在检查 Go 依赖..." -ForegroundColor Yellow
    
    if (Test-Path "go.mod") {
        Write-Host "发现 go.mod 文件，正在整理依赖..." -ForegroundColor Gray
        $tidyResult = Start-Process -FilePath "go" -ArgumentList @("mod", "tidy") -NoNewWindow -Wait -PassThru -WorkingDirectory $(Get-Location)
        if ($tidyResult.ExitCode -eq 0) {
            Write-Host "依赖整理完成" -ForegroundColor Green
        } else {
            Write-Warning "依赖整理失败，将继续构建过程"
        }
    } else {
        Write-Warning "未找到 go.mod 文件，可能需要手动处理依赖"
    }
    
    Write-Host "`n正在构建 AMMDS Launcher..." -ForegroundColor Yellow
    
    Write-Host "清理之前的构建..." -ForegroundColor Gray
    go clean
    
    Write-Host "生成 Windows 资源文件..." -ForegroundColor Gray
    if (Test-Path "icon.ico") {
        rsrc -ico icon.ico -o rsrc.syso
        Write-Host "已生成资源文件 rsrc.syso" -ForegroundColor Cyan
    } else {
        Write-Warning "未找到 icon.ico 文件，将使用默认图标"
    }
    
    Write-Host "开始构建 Windows GUI 应用..." -ForegroundColor Gray
    $buildArgs = @(
        "build",
        "-v",
        "-ldflags", '"-H windowsgui"',
        "-o", "$outputDir\AMMDS-Launcher.exe"
    )
    
    Write-Host "执行命令: go $buildArgs" -ForegroundColor Gray
    $startTime = Get-Date
    $result = Start-Process -FilePath "go" -ArgumentList $buildArgs -NoNewWindow -Wait -PassThru -WorkingDirectory $(Get-Location)
    $endTime = Get-Date
    $duration = ($endTime - $startTime).TotalSeconds
    
    Write-Host "构建耗时: $duration 秒" -ForegroundColor Gray
    
    if ($result.ExitCode -eq 0) {
        Write-Host "构建成功!" -ForegroundColor Green
        
        if (Test-Path "$outputDir\AMMDS-Launcher.exe") {
            $exeInfo = Get-Item "$outputDir\AMMDS-Launcher.exe"
            Write-Host "输出文件: $($exeInfo.FullName)" -ForegroundColor Cyan
            Write-Host "文件大小: $([math]::Round($exeInfo.Length / 1KB, 2)) KB" -ForegroundColor Cyan
        } else {
            Write-Error "构建过程声称成功，但未找到输出文件 AMMDS-Launcher.exe"
            exit 1
        }
        
        Write-Host "复制依赖文件..." -ForegroundColor Gray
        $requiredFiles = @("ammds.exe", "icon.ico", "logo.ico")
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

if ((-not $NoBuild) -and (-not (Test-Path "$outputDir\AMMDS-Launcher.exe"))) {
    Write-Error "AMMDS-Launcher.exe 文件不存在，无法创建安装包"
    exit 1
} else {
    Write-Host "`n跳过构建步骤..." -ForegroundColor Yellow
}

if (-not $NoInstaller) {
    if (-not (Test-Path "$outputDir\AMMDS-Launcher.exe")) {
        Write-Error "AMMDS-Launcher.exe 不存在，无法创建安装包"
        exit 1
    }
    
    Write-Host "`n正在检查 Inno Setup 编译器..." -ForegroundColor Yellow
    
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
    
    # Try to find ISCC.exe in PATH if not found in common locations
    if (-not $isccPath) {
        try {
            $isccCommand = Get-Command ISCC.exe -ErrorAction Stop
            $isccPath = $isccCommand.Source
        } catch {
            Write-Host "未能在常见位置或PATH中找到ISCC.exe" -ForegroundColor Red
        }
    }
    
    if ($isccPath) {
        Write-Host "找到 Inno Setup 编译器: $isccPath" -ForegroundColor Cyan
        
        $issFile = "installer.iss"
        if (Test-Path $issFile) {
            $content = Get-Content $issFile
            $content = $content -replace 'AppVersion=.*', "AppVersion=$Version"
            Set-Content $issFile $content
            
            Write-Host "已更新安装脚本版本号为: $Version" -ForegroundColor Cyan
            
            Write-Host "`n正在编译安装包..." -ForegroundColor Yellow
            
            $issArgs = @(
                "$issFile"
            )
            
            Write-Host "执行命令: $isccPath $issArgs" -ForegroundColor Gray
            $startTime = Get-Date
            $result = Start-Process -FilePath $isccPath -ArgumentList $issArgs -NoNewWindow -Wait -PassThru -WorkingDirectory $(Get-Location)
            $endTime = Get-Date
            $duration = ($endTime - $startTime).TotalSeconds
            
            Write-Host "安装包编译耗时: $duration 秒" -ForegroundColor Gray
            
            if ($result.ExitCode -eq 0) {
                Write-Host "安装包编译成功!" -ForegroundColor Green
                
                $setupFiles = Get-ChildItem -Path "dist" -Filter "*.exe" -ErrorAction SilentlyContinue | Where-Object {$_.Name -like "*setup*"}
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
                exit $result.ExitCode
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