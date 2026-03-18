# CFping

一个带网页界面的 Cloudflare IP 检测工具，用来测试：

1. TCP 连通性和延迟
2. 指定下载地址的测速结果

当前项目已经补上第一版开发规则、自动测试和一键检查入口，后续更适合按“先写验收，再让 AI 改代码”的方式维护。

## 当前能力

- 支持单个 IP 输入
- 支持 CIDR 输入，例如 `1.1.1.0/24`
- 通过 WebSocket 实时返回扫描结果
- 支持 `ping` 和下载测速两种模式
- 结果表格支持排序
- 默认启动后自动打开浏览器

## 快速启动

### 环境要求

- Go 1.24+

### 本地运行

```powershell
go run .
```

Windows 也可以直接运行：

```powershell
.\run.bat
```

启动后访问：

```text
http://localhost:13334
```

## 一键检查

以后开发默认不要只靠手工点页面。

统一检查命令：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\check.ps1
```

这条命令会依次执行：

1. `go test ./...`
2. `go build ./...`

只有它通过，才说明这次改动至少没有破坏当前的基础能力。

## 使用方式

1. 在输入框中填入 IP 或 CIDR，每行一个
2. 端口默认是 `443`
3. `Max Ping` 用来过滤过慢结果
4. 点击 `Start Ping` 或 `Start Speed Test`
5. 在表格里查看实时返回结果

默认测速地址：

```text
https://speed.cloudflare.com/__down?bytes=25000000
```

## 项目结构

```text
cfIP/
├─ main.go                  # HTTP 服务入口和 WebSocket 处理
├─ request_logic.go         # 可测试的请求处理规则
├─ scanner/                 # Ping 和测速实现
├─ static/                  # 前端页面
├─ scripts/check.ps1        # 一键检查入口
├─ docs/                    # 项目目标、验收标准、开发规则
└─ *_test.go                # 自动回归测试
```

## 推荐先看的文档

- `docs/01-项目目标与验收.md`
- `docs/02-架构与开发规则.md`

如果你后面继续让 AI 参与开发，先看这两个文件，再下修改任务，效果会稳定很多。
