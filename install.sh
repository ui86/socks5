#!/bin/bash

# SOCKS5 服务器安装脚本
# 支持 linux/amd64 和 linux/arm64 架构
# 功能：安装、卸载、修改配置

# 默认参数
DEFAULT_PORT=1080
DEFAULT_USER=""
DEFAULT_PASSWORD=""
DEFAULT_WHITELIST=""
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="socks5"
GITHUB_REPO="ui86/socks5"

# 颜色定义
RED="\033[31m"
GREEN="\033[32m"
YELLOW="\033[33m"
BLUE="\033[34m"
RESET="\033[0m"

# 显示帮助信息
show_help() {
    echo -e "${BLUE}SOCKS5 服务器安装脚本${RESET}"
    echo "使用方法: 运行脚本后根据提示进行交互操作"
    echo ""
    echo "此脚本支持以下操作："
    echo "  1. 安装SOCKS5服务器"
    echo "  2. 卸载SOCKS5服务器"
    echo "  3. 修改SOCKS5服务器配置"
    echo ""
    echo "注意：此脚本需要以root用户或使用sudo运行"
    exit 0
}

# 检测系统架构
check_architecture() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64) echo "amd64" ;;
        aarch64) echo "arm64" ;;
        *) echo -e "${RED}不支持的系统架构: $arch${RESET}" >&2 ; exit 1 ;;
    esac
}

# 获取最新版本号
get_latest_version() {
    echo "正在获取最新版本号..."
    local latest_version
    # 使用GitHub API获取最新版本号
    latest_version=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep -oP '"tag_name": "\K(.*?)(?=")')
    if [ -z "$latest_version" ]; then
        echo -e "${RED}获取最新版本号失败，使用默认版本 1.0.0${RESET}"
        latest_version="1.0.0"
    fi
    echo "最新版本: $latest_version"
    echo "$latest_version"
}

# 下载并安装二进制文件
download_and_install() {
    local version=$1
    local arch=$2
    # shellcheck disable=SC2155
    local temp_dir=$(mktemp -d)
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${version}/socks5-${version}-linux-${arch}.tar.gz"
    local tar_file="${temp_dir}/socks5-${version}-linux-${arch}.tar.gz"
    
    echo "正在下载 ${download_url}..."
    if ! curl -L -o "${tar_file}" "${download_url}"; then
        echo -e "${RED}下载失败，请检查网络连接或版本是否存在${RESET}"
        rm -rf "${temp_dir}"
        exit 1
    fi
    
    echo "解压安装包..."
    if ! tar -xzf "${tar_file}" -C "${temp_dir}"; then
        echo -e "${RED}解压失败${RESET}"
        rm -rf "${temp_dir}"
        exit 1
    fi
    
    echo "安装到 ${INSTALL_DIR}..."
    if ! sudo mv "${temp_dir}/socks5" "${INSTALL_DIR}"; then
        echo -e "${RED}安装失败，请检查权限${RESET}"
        rm -rf "${temp_dir}"
        exit 1
    fi
    
    sudo chmod +x "${INSTALL_DIR}/socks5"
    rm -rf "${temp_dir}"
    echo -e "${GREEN}安装成功${RESET}"
}

# 创建systemd服务文件
create_systemd_service() {
    local port=$1
    local user=$2
    local password=$3
    local whitelist=$4
    local service_file="/etc/systemd/system/${SERVICE_NAME}.service"
    
    echo "正在创建systemd服务文件..."
    
    # 构建命令参数
    local cmd_args="-p ${port}"
    if [ -n "${user}" ] && [ -n "${password}" ]; then
        cmd_args="${cmd_args} -user ${user} -pwd ${password}"
    fi
    if [ -n "${whitelist}" ]; then
        cmd_args="${cmd_args} --whitelist ${whitelist}"
    fi
    
    # 创建服务文件
    if ! sudo tee "${service_file}" > /dev/null << EOF
[Unit]
Description=SOCKS5 Proxy Server
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/socks5 ${cmd_args}
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    then
        echo -e "${RED}创建服务文件失败，请检查权限${RESET}"
        exit 1
    fi
    
    # 重新加载systemd配置
    sudo systemctl daemon-reload
    
    # 启用服务（开机自启）
    sudo systemctl enable "${SERVICE_NAME}"
    
    echo -e "${GREEN}systemd服务创建成功${RESET}"
}

# 启动服务
start_service() {
    echo "正在启动服务..."
    if ! sudo systemctl start "${SERVICE_NAME}"; then
        echo -e "${RED}启动服务失败，请运行 'systemctl status ${SERVICE_NAME}' 查看详情${RESET}"
        exit 1
    fi
    
    echo -e "${GREEN}服务启动成功${RESET}"
    echo "服务状态:"
    sudo systemctl status "${SERVICE_NAME}" --no-pager

    echo -e "${BLUE}SOCKS5 服务器安装完成！${RESET}"

    echo "配置信息:"
    echo "  端口: ${port}"
    echo "  认证: $(if [ -n "${user}" ] && [ -n "${password}" ]; then echo "启用"; else echo "禁用"; fi)"
    echo "  白名单: $(if [ -n "${whitelist}" ]; then echo "${whitelist}"; else echo "无（允许所有IP）"; fi)"

    echo -e "${YELLOW}管理命令:${RESET}"
    echo "  查看服务状态: systemctl status ${SERVICE_NAME}"
    echo "  停止服务: systemctl stop ${SERVICE_NAME}"
    echo "  重启服务: systemctl restart ${SERVICE_NAME}"
    echo "  禁用开机自启: systemctl disable ${SERVICE_NAME}"
}

# 修改配置函数
modify() {
    local service_file="/etc/systemd/system/${SERVICE_NAME}.service"
    
    # 停止服务
    echo "正在停止服务..."
    sudo systemctl stop "${SERVICE_NAME}" 2>/dev/null
    
    # 更新systemd服务文件
    create_systemd_service "${port}" "${user}" "${password}" "${whitelist}"
    
    # 重启服务
    echo "正在重启服务..."
    if ! sudo systemctl restart "${SERVICE_NAME}"; then
        echo -e "${RED}重启服务失败，请运行 'systemctl status ${SERVICE_NAME}' 查看详情${RESET}"
        exit 1
    fi
    
    echo -e "${GREEN}SOCKS5 服务器配置修改完成！${RESET}"
}



# 卸载函数
uninstall() {
    echo -e "${BLUE}=== SOCKS5 服务器卸载 ===${RESET}"
    
    # 停止服务
    echo "正在停止服务..."
    sudo systemctl stop "${SERVICE_NAME}" 2>/dev/null
    
    # 禁用服务
    echo "正在禁用服务..."
    sudo systemctl disable "${SERVICE_NAME}" 2>/dev/null
    
    # 删除服务文件
    local service_file="/etc/systemd/system/${SERVICE_NAME}.service"
    if [ -f "${service_file}" ]; then
        echo "正在删除服务文件..."
        sudo rm -f "${service_file}"
        sudo systemctl daemon-reload
    fi
    
    # 删除二进制文件
    local binary_path="${INSTALL_DIR}/socks5"
    if [ -f "${binary_path}" ]; then
        echo "正在删除二进制文件..."
        sudo rm -f "${binary_path}"
    fi
    
    echo -e "${GREEN}SOCKS5 服务器卸载完成${RESET}"
}

# 交互式获取操作类型
get_action() {
    while true; do
        echo -e "${BLUE}=== SOCKS5 服务器管理 ===${RESET}"
        echo "请选择要执行的操作："
        echo "1. 安装SOCKS5服务器"
        echo "2. 卸载SOCKS5服务器"
        echo "3. 修改SOCKS5服务器配置"
        echo "4. 显示帮助信息"
        
        read -p "请输入选项 (1-4): " choice
        
        case $choice in
            1)
                action="install"
                break
                ;;
            2)
                action="uninstall"
                break
                ;;
            3)
                action="modify"
                break
                ;;
            4)
                show_help
                ;;
            *)
                echo -e "${RED}无效的选项，请重新输入${RESET}"
                ;;
        esac
    done
}

# 交互式获取安装配置参数
get_install_config() {
    # 重置参数为默认值
    port="${DEFAULT_PORT}"
    user="${DEFAULT_USER}"
    password="${DEFAULT_PASSWORD}"
    whitelist="${DEFAULT_WHITELIST}"
    version=""
    
    # 获取端口号
    while true; do
        read -p "请输入SOCKS5服务器端口号 [默认: ${DEFAULT_PORT}]: " input_port
        if [ -z "${input_port}" ]; then
            port=${DEFAULT_PORT}
            break
        elif [[ "${input_port}" =~ ^[0-9]+$ ]] && [ "${input_port}" -ge 1 ] && [ "${input_port}" -le 65535 ]; then
            port=${input_port}
            break
        else
            echo -e "${RED}无效的端口号，请输入1-65535之间的数字${RESET}"
        fi
    done
    
    # 获取是否启用认证
    while true; do
        read -p "是否启用用户认证? (y/n) [默认: n]: " enable_auth
        enable_auth=${enable_auth:-n}
        
        case ${enable_auth} in
            [Yy])
                read -p "请输入用户名: " user
                while [ -z "${user}" ]; do
                    read -p "用户名不能为空，请重新输入: " user
                done
                
                # shellcheck disable=SC2162
                read -s -p "请输入密码: " password
                echo
                while [ -z "${password}" ]; do
                    read -s -p "密码不能为空，请重新输入: " password
                    echo
                done
                break
                ;;
            [Nn])
                user=""
                password=""
                break
                ;;
            *)
                echo -e "${RED}无效的选项，请输入y或n${RESET}"
                ;;
        esac
    done
    
    # 获取IP白名单
    read -p "请输入IP白名单（多个IP用逗号分隔，留空表示不限制）: " whitelist
    
    # 获取版本信息
    read -p "请输入要安装的版本（留空表示最新版本）: " version
    
    # 显示确认信息
    echo -e "${GREEN}安装配置确认：${RESET}"
    echo "端口: ${port}"
    echo "认证: $(if [ -n "${user}" ] && [ -n "${password}" ]; then echo "启用 (用户名: ${user})"; else echo "禁用"; fi)"
    echo "白名单: $(if [ -n "${whitelist}" ]; then echo "${whitelist}"; else echo "无（允许所有IP）"; fi)"
    echo "版本: $(if [ -n "${version}" ]; then echo "${version}"; else echo "最新版本"; fi)"
    
    while true; do
        read -p "确认以上配置是否正确? (y/n) [默认: y]: " confirm
        confirm=${confirm:-y}
        
        case ${confirm} in
            [Yy])
                break
                ;;
            [Nn])
                echo -e "${YELLOW}配置已取消，将重新开始${RESET}"
                get_install_config
                break
                ;;
            *)
                echo -e "${RED}无效的选项，请输入y或n${RESET}"
                ;;
        esac
    done
}

# 交互式获取修改配置参数
get_modify_config() {
    local service_file="/etc/systemd/system/${SERVICE_NAME}.service"
    
    # 检查服务是否存在
    if [ ! -f "${service_file}" ]; then
        echo -e "${RED}SOCKS5 服务未安装，请先安装服务${RESET}"
        exit 1
    fi
    
    # 尝试获取当前配置
    echo "正在读取当前配置..."
    # shellcheck disable=SC2155
    local current_cmd=$(sudo cat "${service_file}" | grep -oP 'ExecStart=/usr/local/bin/socks5 \K.*')
    
    # 解析当前参数作为默认值
    local current_port=${DEFAULT_PORT}
    local current_user=${DEFAULT_USER}
    local current_password=${DEFAULT_PASSWORD}
    local current_whitelist=${DEFAULT_WHITELIST}
    
    # 从当前命令行解析参数
    if [ -n "${current_cmd}" ]; then
        # 解析端口
        current_port=$(echo "${current_cmd}" | grep -oP '-p \K\d+')
        [ -z "${current_port}" ] && current_port=${DEFAULT_PORT}
        
        # 解析用户名
        current_user=$(echo "${current_cmd}" | grep -oP '-user \K\S+')
        
        # 解析密码（不显示当前密码）
        if [ -n "${current_user}" ]; then
            # shellcheck disable=SC2034
            current_password="******"
        fi
        
        # 解析白名单
        current_whitelist=$(echo "${current_cmd}" | grep -oP '--whitelist \K[^ ]+')
    fi
    
    # 显示当前配置
    echo -e "${GREEN}当前配置:${RESET}"
    echo "  端口: ${current_port}"
    echo "  认证: $(if [ -n "${current_user}" ]; then echo "启用 (用户名: ${current_user})"; else echo "禁用"; fi)"
    echo "  白名单: $(if [ -n "${current_whitelist}" ]; then echo "${current_whitelist}"; else echo "无"; fi)"
    
    # 获取新的配置参数
    echo -e "${BLUE}请输入新的配置参数（留空表示保持当前配置）${RESET}"
    
    # 获取端口号
    while true; do
        read -p "请输入SOCKS5服务器端口号 [当前: ${current_port}]: " input_port
        if [ -z "${input_port}" ]; then
            port=${current_port}
            break
        elif [[ "${input_port}" =~ ^[0-9]+$ ]] && [ "${input_port}" -ge 1 ] && [ "${input_port}" -le 65535 ]; then
            port=${input_port}
            break
        else
            echo -e "${RED}无效的端口号，请输入1-65535之间的数字${RESET}"
        fi
    done
    
    # 获取是否启用认证
    while true; do
        read -p "是否修改用户认证设置? (y/n) [默认: n]: " change_auth
        change_auth=${change_auth:-n}
        
        case ${change_auth} in
            [Yy])
                while true; do
                    read -p "是否启用用户认证? (y/n): " enable_auth
                    case ${enable_auth} in
                        [Yy])
                            read -p "请输入用户名: " user
                            while [ -z "${user}" ]; do
                                read -p "用户名不能为空，请重新输入: " user
                            done
                            
                            read -s -p "请输入密码: " password
                            echo
                            while [ -z "${password}" ]; do
                                read -s -p "密码不能为空，请重新输入: " password
                                echo
                            done
                            break
                            ;;
                        [Nn])
                            user=""
                            password=""
                            break
                            ;;
                        *)
                            echo -e "${RED}无效的选项，请输入y或n${RESET}"
                            ;;
                    esac
                done
                break
                ;;
            [Nn])
                # 保持当前设置（注意密码不会保留，需要重新输入如果启用了认证）
                if [ -n "${current_user}" ]; then
                    user=${current_user}
                    read -s -p "请重新输入密码 (留空表示使用当前密码): " input_password
                    echo
                    if [ -z "${input_password}" ]; then
                        # 如果留空，保持当前密码，通过重新读取服务文件获取
                        # shellcheck disable=SC2155
                        local current_password_cmd=$(sudo cat "${service_file}" | grep -oP '-pwd \K\S+')
                        password=${current_password_cmd}
                    else
                        password=${input_password}
                    fi
                else
                    user=""
                    password=""
                fi
                break
                ;;
            *)
                echo -e "${RED}无效的选项，请输入y或n${RESET}"
                ;;
        esac
    done
    
    # 获取IP白名单
    read -p "请输入IP白名单（多个IP用逗号分隔，留空表示保持当前设置）: " input_whitelist
    if [ -z "${input_whitelist}" ]; then
        whitelist=${current_whitelist}
    else
        whitelist=${input_whitelist}
    fi
    
    # 显示新配置
    echo -e "${GREEN}新配置:${RESET}"
    echo "  端口: ${port}"
    echo "  认证: $(if [ -n "${user}" ] && [ -n "${password}" ]; then echo "启用 (用户名: ${user})"; else echo "禁用"; fi)"
    echo "  白名单: $(if [ -n "${whitelist}" ]; then echo "${whitelist}"; else echo "无"; fi)"
    
    # 确认修改
    while true; do
        read -p "确认修改配置? (y/n) [默认: y]: " confirm
        confirm=${confirm:-y}
        
        case ${confirm} in
            [Yy])
                break
                ;;
            [Nn])
                echo -e "${YELLOW}配置修改已取消${RESET}"
                exit 0
                ;;
            *)
                echo -e "${RED}无效的选项，请输入y或n${RESET}"
                ;;
        esac
    done
}

# 主函数
main() {
    # 检查是否为root用户
    if [ "$(id -u)" != "0" ]; then
        echo -e "${RED}请以root用户或使用sudo运行此脚本${RESET}"
        exit 1
    fi
    
    # 检查依赖工具
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}未找到curl命令，请先安装curl${RESET}"
        exit 1
    fi
    
    if ! command -v systemctl &> /dev/null; then
        echo -e "${RED}未找到systemctl命令，此脚本仅支持使用systemd的Linux系统${RESET}"
        exit 1
    fi
    
    # 交互式获取操作类型
    get_action
    
    # 根据操作类型执行相应的功能
    case "${action}" in
        install)
            echo -e "${BLUE}=== SOCKS5 服务器安装 ===${RESET}"
            
            # 交互式获取安装配置
            get_install_config
            
            # 检测系统架构
            arch=$(check_architecture)
            echo "系统架构: $arch"
            
            # 获取版本号
            if [ -z "${version}" ]; then
                version=$(get_latest_version)
            else
                echo "指定版本: ${version}"
            fi
            
            # 下载并安装
            download_and_install "${version}" "${arch}"
            
            # 创建systemd服务
            create_systemd_service "${port}" "${user}" "${password}" "${whitelist}"
            
            # 启动服务
            start_service
            ;;
        uninstall)
            # 确认卸载
            while true; do
                read -p "确定要卸载SOCKS5服务器吗? 此操作将删除所有相关文件和配置。(y/n) [默认: n]: " confirm
                confirm=${confirm:-n}
                
                case ${confirm} in
                    [Yy])
                        uninstall
                        break
                        ;;
                    [Nn])
                        echo -e "${YELLOW}卸载已取消${RESET}"
                        exit 0
                        ;;
                    *)
                        echo -e "${RED}无效的选项，请输入y或n${RESET}"
                        ;;
                esac
            done
            ;;
        modify)
            echo -e "${BLUE}=== SOCKS5 服务器配置修改 ===${RESET}"
            
            # 交互式获取修改配置
            get_modify_config
            
            # 执行修改操作
            modify
            ;;
        *)
            echo -e "${RED}未知操作: ${action}${RESET}"
            show_help
            ;;
    esac
}

# 执行主函数
main "$@"