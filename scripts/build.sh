#!/bin/bash
# 编译脚本：编译CloudflareSpeedTest到target平台

set -e

# 默认参数
GOOS="linux"
GOARCH="arm64"
OUTPUT_NAME="cfst"
VERSION=""

# 函数：显示帮助
usage() {
    echo "CloudflareSpeedTest 编译脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -o, --os OS             目标操作系统 (默认: linux)"
    echo "  -a, --arch ARCH         目标架构 (默认: arm64)"
    echo "                          支持: amd64, arm64, arm, 386"
    echo "  -n, --name NAME         输出文件名 (默认: cfst)"
    echo "  -v, --version VERSION   版本号 (可选)"
    echo "  -h, --help              显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  # 编译到 Linux ARM64"
    echo "  $0"
    echo ""
    echo "  # 编译到 Linux AMD64"
    echo "  $0 -a amd64"
    echo ""
    echo "  # 编译到 macOS ARM64 (Apple Silicon)"
    echo "  $0 -o darwin -a arm64"
    echo ""
    echo "  # 编译到 Windows AMD64"
    echo "  $0 -o windows -a amd64 -n cfst.exe"
}

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -o|--os)
            GOOS="$2"
            shift 2
            ;;
        -a|--arch)
            GOARCH="$2"
            shift 2
            ;;
        -n|--name)
            OUTPUT_NAME="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "未知选项: $1"
            usage
            exit 1
            ;;
    esac
done

# 验证架构
case "$GOARCH" in
    amd64|arm64|arm|386)
        ;;
    *)
        echo "错误: 不支持的架构 '$GOARCH'"
        echo "支持的架构: amd64, arm64, arm, 386"
        exit 1
        ;;
esac

# 显示编译信息
echo "================================"
echo "编译 CloudflareSpeedTest"
echo "================================"
echo "操作系统: $GOOS"
echo "架构:    $GOARCH"
echo "输出文件: $OUTPUT_NAME"
if [ -n "$VERSION" ]; then
    echo "版本号:   $VERSION"
fi
echo ""

# 设置环境变量
export GOOS
export GOARCH
export CGO_ENABLED=0

# 构建编译命令
BUILD_CMD="go build -trimpath -ldflags '-s -w'"

# 如果提供了版本号，添加到ldflags
if [ -n "$VERSION" ]; then
    BUILD_CMD="go build -trimpath -ldflags '-s -w -X main.version=$VERSION'"
fi

BUILD_CMD="$BUILD_CMD -o $OUTPUT_NAME main.go"

echo "执行编译命令..."
echo "$ $BUILD_CMD"
echo ""

# 执行编译
eval "$BUILD_CMD"

# 检查编译结果
if [ -f "$OUTPUT_NAME" ]; then
    FILE_SIZE=$(du -h "$OUTPUT_NAME" | cut -f1)
    echo "================================"
    echo "✓ 编译成功!"
    echo "================================"
    echo "输出文件: $OUTPUT_NAME"
    echo "文件大小: $FILE_SIZE"
    echo "目标平台: $GOOS/$GOARCH"
    echo ""
    echo "使用方式:"
    echo "  scp $OUTPUT_NAME user@target-host:/opt/cfst/"
    echo "  ssh user@target-host '/opt/cfst/$OUTPUT_NAME'"
else
    echo "✗ 编译失败!"
    exit 1
fi
