#!/bin/bash
# 部署脚本：将 result.html 复制到 Nginx 静态文件目录

set -e

CONFIG_FILE="instance/config.yaml"
DIST_DIR="dist"
RESULT_HTML="${DIST_DIR}/result.html"

# 函数：从YAML中读取值
yaml_get() {
    grep "^$1:" "$CONFIG_FILE" | sed "s/.*: *//" | tr -d '"' | tr -d "'"
}

# 函数：显示帮助
usage() {
    echo "CloudflareSpeedTest 部署脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -c, --config FILE       配置文件路径 (默认: instance/config.yaml)"
    echo "  -t, --target DIR        Nginx 目标目录 (覆盖配置文件中的nginx_dir)"
    echo "  -h, --help              显示此帮助信息"
    echo ""
    echo "说明:"
    echo "  - 脚本会尝试读取 config.yaml 中的 nginx_dir 配置"
    echo "  - 如果配置了 nginx_dir 或使用 -t 选项，会自动复制 result.html"
    echo "  - 不需要重启 Nginx，文件更新会立即生效"
    echo ""
    echo "示例:"
    echo "  $0                      # 使用配置文件中的 nginx_dir"
    echo "  $0 -t /var/www/html     # 复制到指定目录"
    echo "  $0 -c /etc/cfst/config.yaml -t /app/static"
}

# 解析参数
NGINX_DIR=""
while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--config)
            CONFIG_FILE="$2"
            shift 2
            ;;
        -t|--target)
            NGINX_DIR="$2"
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

# 检查配置文件
if [ ! -f "$CONFIG_FILE" ]; then
    echo "× 错误: 配置文件不存在 ($CONFIG_FILE)"
    exit 1
fi

# 检查 result.html
if [ ! -f "$RESULT_HTML" ]; then
    echo "× 错误: result.html 不存在 ($RESULT_HTML)"
    echo "请先运行测速：./cfst"
    exit 1
fi

# 如果没有通过参数指定，从配置文件读取 nginx_dir
if [ -z "$NGINX_DIR" ]; then
    NGINX_DIR=$(yaml_get "nginx_dir" 2>/dev/null || true)
fi

# 如果仍然为空，显示信息并退出
if [ -z "$NGINX_DIR" ]; then
    echo "⊘ 信息: 未配置 nginx_dir，跳过部署"
    echo ""
    echo "如要部署到 Nginx，请选择以下方式其一："
    echo "  1. 编辑 instance/config.yaml，设置 nginx_dir 的值"
    echo "  2. 运行: $0 -t /path/to/nginx/html"
    exit 0
fi

# 检查目标目录
if [ ! -d "$NGINX_DIR" ]; then
    echo "× 错误: Nginx 目录不存在 ($NGINX_DIR)"
    exit 1
fi

# 复制文件
echo "部署 result.html"
echo "================="
echo "来源:  $RESULT_HTML"
echo "目标:  $NGINX_DIR/"
echo ""

cp "$RESULT_HTML" "$NGINX_DIR/result.html"

# 验证复制
if [ -f "$NGINX_DIR/result.html" ]; then
    MTIME=$(stat -f "%Sm" -t "%Y-%m-%d %H:%M:%S" "$NGINX_DIR/result.html" 2>/dev/null || stat -c %y "$NGINX_DIR/result.html" 2>/dev/null | cut -d. -f1)
    echo "✓ 部署成功!"
    echo ""
    echo "更新时间: $MTIME"
    echo "文件位置: $NGINX_DIR/result.html"
    echo ""
    echo "说明:"
    echo "  - Nginx 会自动读取最新的 result.html 文件"
    echo "  - 无需重启 Nginx 服务"
    echo "  - 访问 Nginx 即可看到最新的测速结果"
else
    echo "× 部署失败!"
    exit 1
fi
