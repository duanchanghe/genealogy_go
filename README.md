# Genealogy Go

一个使用 Go 语言开发的家谱管理系统。

## 功能特点

- 家族成员信息管理
- 家族关系图谱展示
- 家族历史记录
- 家族事件记录
- RESTful API 接口

## 技术栈

- Go 1.21+
- Gin Web 框架
- GORM
- PostgreSQL/MySQL
- Docker (可选)

## 项目结构

```
.
├── cmd/                    # 主程序入口
├── internal/              # 内部包
│   ├── api/              # API 处理器
│   ├── model/            # 数据模型
│   ├── repository/       # 数据访问层
│   └── service/          # 业务逻辑层
├── pkg/                  # 公共包
├── configs/              # 配置文件
└── docs/                 # 文档
```

## 快速开始

1. 克隆项目
```bash
git clone https://github.com/yourusername/genealogy_go.git
cd genealogy_go
```

2. 安装依赖
```bash
go mod download
```

3. 配置数据库
```bash
cp configs/config.example.yaml configs/config.yaml
# 编辑 config.yaml 文件，配置数据库连接信息
```

4. 运行项目
```bash
go run cmd/main.go
```

## API 文档

API 文档将在项目完成后提供。

## 贡献指南

欢迎提交 Issue 和 Pull Request。

## 许可证

MIT License
