#!/bin/bash

# chat-matcher Docker 构建和运行脚本

set -e

PROJECT_NAME="chat-matcher"
IMAGE_NAME="chat-matcher:latest"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印信息函数
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 显示帮助信息
show_help() {
    echo "Usage: $0 [OPTION]"
    echo "Options:"
    echo "  build     构建 Docker 镜像"
    echo "  run       运行容器"
    echo "  stop      停止容器"
    echo "  restart   重启容器"
    echo "  logs      查看容器日志"
    echo "  clean     清理镜像和容器"
    echo "  help      显示此帮助信息"
}

# 构建镜像
build_image() {
    print_info "开始构建 Docker 镜像..."
    docker build -t $IMAGE_NAME .
    print_info "镜像构建完成: $IMAGE_NAME"
}

# 运行容器
run_container() {
    # 检查容器是否已存在
    if docker ps -a --format '{{.Names}}' | grep -q "^${PROJECT_NAME}$"; then
        print_warning "容器 $PROJECT_NAME 已存在"
        docker rm -f $PROJECT_NAME
    fi
    
    print_info "启动容器..."
    docker run -d \
        --name $PROJECT_NAME \
        -p 8080:8080 \
        --restart unless-stopped \
        $IMAGE_NAME
    
    print_info "容器已启动，访问地址: http://localhost:8080/static/index.html"
}

# 停止容器
stop_container() {
    if docker ps --format '{{.Names}}' | grep -q "^${PROJECT_NAME}$"; then
        print_info "停止容器 $PROJECT_NAME..."
        docker stop $PROJECT_NAME
        print_info "容器已停止"
    else
        print_warning "容器 $PROJECT_NAME 未运行"
    fi
}

# 重启容器
restart_container() {
    stop_container
    sleep 2
    run_container
}

# 查看日志
show_logs() {
    if docker ps -a --format '{{.Names}}' | grep -q "^${PROJECT_NAME}$"; then
        print_info "显示容器日志..."
        docker logs -f $PROJECT_NAME
    else
        print_error "容器 $PROJECT_NAME 不存在"
    fi
}

# 清理
clean_up() {
    print_info "清理容器和镜像..."
    
    # 停止并删除容器
    if docker ps -a --format '{{.Names}}' | grep -q "^${PROJECT_NAME}$"; then
        docker rm -f $PROJECT_NAME
        print_info "已删除容器 $PROJECT_NAME"
    fi
    
    # 删除镜像
    if docker images --format '{{.Repository}}:{{.Tag}}' | grep -q "^${IMAGE_NAME}$"; then
        docker rmi $IMAGE_NAME
        print_info "已删除镜像 $IMAGE_NAME"
    fi
    
    print_info "清理完成"
}

# 主逻辑
case "$1" in
    build)
        build_image
        ;;
    run)
        run_container
        ;;
    stop)
        stop_container
        ;;
    restart)
        restart_container
        ;;
    logs)
        show_logs
        ;;
    clean)
        clean_up
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "未知参数: $1"
        show_help
        exit 1
        ;;
esac