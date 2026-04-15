# 脚本说明

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

## deploy-html.sh

部署脚本，将生成的 `result.html` 复制到 Nginx 静态文件目录。

### 使用方法

```bash
# 使用 config.yaml 中配置的 nginx_dir
./scripts/deploy-html.sh

# 复制到指定目录（覆盖配置）
./scripts/deploy-html.sh -t /var/www/html

# 使用自定义配置文件
./scripts/deploy-html.sh -c /etc/cfst/config.yaml -t /app/static
```

### 参数说明

- `-c, --config FILE` - 配置文件路径 (默认: `instance/config.yaml`)
- `-t, --target DIR` - Nginx 目标目录 (覆盖配置文件中的 `nginx_dir`)
- `-h, --help` - 显示帮助信息

### 配置方式

在 `instance/config.yaml` 中配置 Nginx 目录：

```yaml
# Nginx 静态文件目录（可选）
# 示例: /app/static, /var/www/html, /usr/share/nginx/html
nginx_dir: "/var/www/html"
```

### 重要说明

- **无需重启 Nginx** - 只是复制文件，Nginx 会自动读取更新
- 文件系统更新立即生效
- 访问 Nginx 即可看到最新的测速结果

---

## 完整工作流示例

### 1. 首次配置

```bash
# 复制配置模板
cp config.template.yaml instance/config.yaml

# 编辑配置（按需修改）
nano instance/config.yaml
```

### 2. 编译

```bash
# 编译到 Linux ARM64
./scripts/build.sh

# 或编译到其他平台
./scripts/build.sh -a amd64
./scripts/build.sh -o darwin -a arm64
```

### 3. 运行测速

```bash
# 直接运行（会输出到 data/ 和 dist/ 目录）
./cfst

# 或指定 IP 文件
./cfst -f data/ip.txt
```

### 4. 部署到 Nginx

```bash
# 复制 result.html 到 Nginx
./scripts/deploy-html.sh -t /var/www/html

# 或使用配置文件配置
# 编辑 instance/config.yaml，设置 nginx_dir
# 然后运行
./scripts/deploy-html.sh
```

---

## 目录结构说明

```
CloudflareSpeedTest/
├── scripts/                    # 脚本目录
│   ├── build.sh               # 编译脚本
│   ├── deploy-html.sh         # 部署脚本
│   └── README.md              # 脚本说明
├── data/                      # 数据目录（由应用生成）
│   ├── ip.txt                # IP 列表
│   └── result_history.json    # 历史记录（JSON 格式）
├── dist/                      # 输出目录
│   └── result.html           # 测速结果 HTML
├── instance/
│   └── config.yaml           # 用户配置（gitignore 忽略）
├── config.template.yaml      # 配置模板
├── cfst                       # 编译生成的可执行文件
└── ...
```

---

## 支持的平台

### 编译支持

- Linux: amd64, arm64, arm (armv7), 386
- macOS: amd64, arm64
- Windows: amd64, 386

### 部署支持

- 任何支持 Nginx 的操作系统
- 需要对目标目录有写权限
