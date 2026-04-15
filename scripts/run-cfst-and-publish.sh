#!/bin/bash
# 一键执行：运行 cfst，然后发布 HTML 到 nginx_dir/index.html

set -e

CONFIG_FILE="instance/config.yaml"
TARGET_DIR=""
CFST_BIN="./cfst"
SOURCE_HTML="dist/result.html"
TARGET_NAME="index.html"

yaml_get() {
    grep "^$1:" "$CONFIG_FILE" | sed "s/.*: *//" | tr -d '"' | tr -d "'"
}

usage() {
    echo "CloudflareSpeedTest 运行并发布脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -c, --config FILE       配置文件路径 (默认: instance/config.yaml)"
    echo "  -t, --target DIR        目标目录 (覆盖配置中的 nginx_dir)"
    echo "  -b, --bin FILE          cfst 可执行文件路径 (默认: ./cfst)"
    echo "  -h, --help              显示帮助"
    echo ""
    echo "行为:"
    echo "  1) 运行 cfst"
    echo "  2) 将 dist/result.html 移动到 <目标目录>/index.html"
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--config)
            CONFIG_FILE="$2"
            shift 2
            ;;
        -t|--target)
            TARGET_DIR="$2"
            shift 2
            ;;
        -b|--bin)
            CFST_BIN="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "未知参数: $1"
            usage
            exit 1
            ;;
    esac
done

if [ ! -f "$CONFIG_FILE" ]; then
    echo "× 错误: 配置文件不存在 ($CONFIG_FILE)"
    exit 1
fi

if [ -z "$TARGET_DIR" ]; then
    TARGET_DIR=$(yaml_get "nginx_dir" 2>/dev/null || true)
fi

if [ -z "$TARGET_DIR" ]; then
    echo "× 错误: 未配置 nginx_dir"
    echo "请在 $CONFIG_FILE 中设置 nginx_dir，或使用 -t 指定"
    exit 1
fi

if [ ! -d "$TARGET_DIR" ]; then
    echo "× 错误: 目标目录不存在 ($TARGET_DIR)"
    exit 1
fi

if [ ! -x "$CFST_BIN" ]; then
    echo "× 错误: cfst 可执行文件不存在或不可执行 ($CFST_BIN)"
    exit 1
fi

echo "[1/2] 运行测速: $CFST_BIN"
"$CFST_BIN"

if [ ! -f "$SOURCE_HTML" ]; then
    echo "× 错误: 运行后未生成 $SOURCE_HTML"
    exit 1
fi

echo "[2/2] 发布页面"
echo "来源: $SOURCE_HTML"
echo "目标: $TARGET_DIR/$TARGET_NAME"

mv "$SOURCE_HTML" "$TARGET_DIR/$TARGET_NAME"

if [ -f "$TARGET_DIR/$TARGET_NAME" ]; then
    echo "✓ 发布成功: $TARGET_DIR/$TARGET_NAME"
    echo "说明: 仅替换静态文件，无需重启 Nginx"
else
    echo "× 发布失败"
    exit 1
fi
