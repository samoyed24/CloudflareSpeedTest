# 编译脚本

## build.sh

通用编译脚本，支持多平台编译。

### 使用方法

```bash
# Linux ARM64（默认）
./scripts/build.sh

# Linux AMD64
./scripts/build.sh -a amd64

# macOS ARM64 (Apple Silicon)
./scripts/build.sh -o darwin -a arm64

# Windows AMD64
./scripts/build.sh -o windows -a amd64 -n cfst.exe

# 自定义版本号
./scripts/build.sh -v 1.0.0
```

### 参数说明

- `-o, --os OS` - 目标操作系统 (默认: `linux`)
- `-a, --arch ARCH` - 目标架构 (默认: `arm64`)
  - 支持: `amd64`, `arm64`, `arm`, `386`
- `-n, --name NAME` - 输出文件名 (默认: `cfst`)
- `-v, --version VERSION` - 版本号 (可选)
- `-h, --help` - 显示帮助信息

### 编译优化

脚本使用以下优化：
- `CGO_ENABLED=0` - 禁用CGO，生成静态二进制
- `-trimpath` - 移除源码路径信息
- `-ldflags '-s -w'` - 移除符号表和调试信息，减小二进制大小

### 部署到远程主机

编译完成后，可以直接部署：

```bash
# 1. 编译到 Linux ARM64
./scripts/build.sh

# 2. 上传到远程主机
scp cfst root@target-host:/opt/cfst/

# 3. 在远程主机上运行
ssh root@target-host '/opt/cfst/cfst'
```

### 支持的平台

- Linux: amd64, arm64, arm (armv7), 386
- macOS: amd64, arm64
- Windows: amd64, 386
- 其他: 根据Go支持的目标平台可扩展
