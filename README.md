# SOCKS5 服务器

一个轻量级、功能完整的SOCKS5代理服务器实现，支持TCP/UDP协议、用户认证以及IP白名单功能。

## 功能特点

- 支持 SOCKS5 协议标准的 CONNECT 和 UDP ASSOCIATE 命令
- 支持无认证和用户名/密码认证方式
- 支持TCP和UDP代理
- 支持IP白名单功能，可以限制允许连接的客户端IP地址
- 可配置连接超时时间

## 安装

### 前提条件

- Go 1.25.0 或更高版本

### 编译安装

```bash
# 克隆仓库
git clone https://github.com/yourusername/socks5.git
cd socks5

# 编译
go build

# 运行
./socks5
```

## 使用方法

### 基本用法

启动默认配置的SOCKS5服务器（监听1080端口，无认证，允许所有IP连接）：

```bash
./socks5
```

### 指定端口

使用 `-p` 参数指定服务器监听端口：

```bash
./socks5 -p 8080
```

### 启用认证

使用 `-user` 和 `-pwd` 参数设置用户名和密码：

```bash
./socks5 -user admin -pwd password123
```

### 启用IP白名单

使用 `--whitelist` 参数指定允许连接的客户端IP地址，多个IP用逗号分隔：

```bash
./socks5 --whitelist 127.0.0.1,192.168.1.100,1.1.1.1
```

### 组合使用

可以同时使用多个参数：

```bash
./socks5 -p 8080 -user admin -pwd password123 --whitelist 127.0.0.1,192.168.1.0/24
```

## 命令行参数说明

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--user` | | 空 | 认证用户名，不设置则不启用认证 |
| `--pwd` | | 空 | 认证密码，与用户名同时设置才生效 |
| `--port` | `-p` | 1080 | 服务器监听端口 |
| `--whitelist` | | 空 | 允许连接的IP地址列表，多个IP用逗号分隔，为空时允许所有IP连接 |

## 项目结构

```
socks5/
├── main.go         # 程序入口
├── pkg/
│   └── socks5/
│       ├── bind.go         # BIND命令实现
│       ├── client.go       # 客户端相关功能
│       ├── client_side.go  # 客户端侧处理逻辑
│       ├── connect.go      # CONNECT命令实现
│       ├── init.go         # 初始化相关功能
│       ├── server.go       # 服务器核心实现
│       ├── server_side.go  # 服务器侧处理逻辑
│       ├── socks5.go       # 协议定义和基础结构
│       ├── udp.go          # UDP相关功能
│       └── util.go         # 工具函数
├── go.mod          # Go模块定义
├── go.sum          # 依赖版本锁定
└── README.md       # 项目文档
```

## 核心功能说明

### 1. 服务器启动流程

- 解析命令行参数
- 创建SOCKS5服务器实例
- 启动TCP和UDP监听器
- 处理客户端连接请求

### 2. 连接处理流程

1. 接收客户端连接
2. 检查客户端IP是否在白名单中（如果启用了白名单）
3. 进行认证协商（无认证或用户名密码认证）
4. 处理客户端请求（CONNECT或UDP ASSOCIATE）
5. 建立与目标服务器的连接并转发数据

### 3. IP白名单功能

白名单功能允许管理员限制只有特定IP地址的客户端可以连接到SOCKS5服务器。当客户端连接时，服务器会检查其IP地址是否在白名单中，只有在白名单中的IP地址才能继续进行认证和请求处理。

## 依赖说明

- [github.com/patrickmn/go-cache](https://github.com/patrickmn/go-cache) - 提供内存缓存功能，用于存储UDP关联信息
- [github.com/txthinking/runnergroup](https://github.com/txthinking/runnergroup) - 提供并发任务管理功能，用于管理TCP和UDP监听器

## 性能与安全

- 服务器使用goroutine处理每个客户端连接，具有良好的并发性能
- 启用认证和白名单功能可以提高服务器安全性
- 可以通过设置超时时间避免空闲连接占用资源

## License

MIT License

## 注意事项

- 如果不设置白名单，服务器将允许所有IP地址连接，请谨慎在公网环境中使用
- 用户名和密码以明文形式传输，请在安全的网络环境中使用或考虑使用TLS加密
- UDP协议本身不提供可靠传输，某些应用场景下可能会出现数据包丢失