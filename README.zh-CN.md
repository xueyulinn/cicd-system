# CI/CD 流水线管理工具

一个基于 Go 的 CLI 工具，用于校验和管理 CI/CD 流水线配置。该工具可解析 YAML 流水线文件、验证其结构与依赖关系，并通过微服务架构在本地执行流水线。

> English version: [README.md](README.md)

## 概览

e-team 项目是一个 CI/CD 流水线管理系统，提供以下能力：

- **流水线校验**：解析并校验 YAML 流水线配置（由 Validation Service 提供）
- **API Gateway**：统一入口，路由到校验、Dry Run 与执行能力
- **Orchestrator Service**：编排流水线运行（阶段、任务、依赖）
- **Worker 层**：执行具体任务（例如容器内执行）并上报状态
- **请求去重**：相同请求复用最早的在途运行，丢弃重复运行请求
- **CLI 接口**：提供 `verify`、`dryrun`、`run`、`report` 命令

## 功能特性

- `verify`：校验单个流水线文件，或递归校验目录中的 YAML 流水线
- 严格校验规则：阶段/任务结构校验、未定义引用检测、环依赖检测
- 错误信息包含文件与行列定位，便于快速排查
- `dryrun`：仅生成执行计划，不实际运行任务（支持 `yaml` / `json` 输出）
- `run`：通过 API Gateway 与 Orchestrator Service 执行流水线
- Git 感知运行：可锁定到具体提交，或在 CLI 中通过分支选择器解析提交
- 在途运行去重：相同请求不会重复创建运行
- 异步编排：就绪任务发布到 RabbitMQ，由 Worker 消费执行
- 并发控制：支持编排发布并发与 Worker 消费并发配置
- Worker 以容器执行任务，支持可选的 CPU/内存资源限制
- `report`：按流水线/运行/阶段/任务粒度查询历史执行结果（支持 `yaml` / `json`）
- 使用 MySQL 8 持久化运行、阶段、任务、状态、Git 元数据与 trace 关联 ID
- 内置可观测性：OpenTelemetry + Prometheus + Loki + Tempo + Grafana
- 支持 Docker Compose 本地部署与 Kubernetes Helm 部署

## 待实现功能

- Artifacts 上传/下载：采集并打包任务声明的产物，存储到制品存储，并在运行结果/报告中提供可下载链接。
- 失败重试/重跑：支持失败任务重试或整条流水线重跑，并保留每次尝试的历史与最终状态。

## 安装

### 前置要求

- Go 1.25.6 或更高版本
- Git

### 源码构建

```bash
# 克隆仓库
git clone https://github.com/xueyulinn/cicd-system.git
cd cicd-system

# 构建（Windows / macOS / Linux）
make build
# 或手动构建：
go build -o bin/cicd ./cmd/cicd

# 安装到 $HOME/bin（可选）
make install
# 或：go install ./cmd/cicd

# 如果 macOS/Linux 下提示 "cicd: command not found"，将 Go bin 目录加入 PATH：
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
source ~/.zshrc
```

默认构建产物为 `bin/cicd`，可选安装到 `$HOME/bin`。

## 快速开始：运行流水线

1. **启动全部服务**（API Gateway、Validation、Orchestrator、Worker、Reporting 及依赖）：

   ```bash
   docker compose --env-file compose.values.env up -d
   ```

2. **构建 CLI**（如尚未构建）：

   ```bash
   make build
   ```

3. **运行流水线**：

   ```bash
   # 运行默认流水线（.pipelines/pipeline.yaml），使用当前分支与最新提交
   ./bin/cicd run --file .pipelines/pipeline.yaml

   # 或按流水线名称运行（从 .pipelines/ 下解析）
   ./bin/cicd run --name pipeline.yaml

   # 指定分支选择器或明确提交
   ./bin/cicd run --file .pipelines/pipeline.yaml --branch main
   ./bin/cicd run --file .pipelines/pipeline.yaml --commit HEAD
   ```

CLI 默认向 API Gateway（`http://localhost:8000`）发请求；可通过 `GATEWAY_URL` 覆盖。

## 使用说明：如何运行并观测流水线

本节说明如何运行示例流水线、观察成功/失败行为，以及如何生成报告。

> `cicd run` 运行约束：可执行流水线必须位于 Git 仓库根目录的 `.pipelines/` 下。  
> 使用 `--file .pipelines/<file>.yaml` 或 `--name <pipeline-name>`（名称同样从 `.pipelines/` 解析）。

### 仓库与示例流水线

- **仓库**：`xueyulinn/cicd-system`（当前仓库）
- **源码目录**：`cmd/` 与 `internal/`
- **流水线定义**：位于 `.pipelines/`，包含：
  - `.pipelines/pipeline.yaml`：完整、可成功执行的示例（`Default Pipeline`）
  - 其他用于演示失败场景的故意错误示例，如 `.pipelines/circular_dependency.yaml`

### 基础命令

```bash
# 校验默认流水线文件（.pipelines/pipeline.yaml）
cicd verify

# 校验指定流水线文件
cicd verify path/to/pipeline.yaml

# 递归校验目录中的 YAML 文件
cicd verify .pipelines/
```

```bash
# Dry Run（展示执行顺序）
cicd dryrun
cicd dryrun path/to/pipeline.yaml
```

```bash
# 运行流水线（服务需先启动）
# 运行目标必须在 <repo-root>/.pipelines/ 下
cicd run --file .pipelines/pipeline.yaml
cicd run --name pipeline.yaml --branch main
cicd run --name pipeline.yaml --commit HEAD
```

```bash
# 查询流水线执行报告
# report 使用流水线名称作为位置参数
cicd report "Default Pipeline" --run 1
cicd report "Default Pipeline" --run 1 --stage build
cicd report "Default Pipeline" --run 1 --stage test --job unit-tests
cicd report "Default Pipeline" --run 1 -f json
```

## 流水线配置格式

工具期望 YAML 结构如下：

```yaml
# 流水线元数据（可选）
pipeline:
  name: "Example Pipeline"

# 阶段定义
stages:
  - build
  - test
  - deploy

# 任务定义
job-name:
  - stage: build
  - image: golang:1.21
  - script:
    - "make build"

another-job:
  - stage: test
  - needs: [job-name]
  - image: golang:1.21
  - script:
    - "make test"
```

### 配置元素

- **pipeline**：流水线元数据（可选）
- **stages**：阶段名称数组（必须先定义后使用）
- **jobs**：任务定义，包含：
  - `stage`：任务所属阶段（必填）
  - `image`：执行镜像（必填）
  - `script`：执行命令（必填）
  - `needs`：依赖任务列表（可选）

## 系统架构

| 组件 | 端口 | 说明 |
|------|------|------|
| API Gateway | 8000 | 路由到校验 / dryrun / 执行 |
| Validation Service | 8001 | 校验流水线 YAML |
| Orchestrator Service | 8002 | 执行编排与调度 |
| Worker Service | 8003 | 执行任务步骤 |
| Reporting Service | 8004 | 运行报告查询 |
| Redis | 6379 | 校验缓存后端 |
| MySQL 8 | 3306 | 报告存储数据库 |
| Prometheus | 9090 | 指标采集与存储 |
| Loki | 3100 | 日志存储与检索 |
| Tempo | 3200 | 链路追踪存储 |
| OTel Collector | 4318 | 遥测数据接收与路由 |
| Grafana | 3000 | 统一可观测性界面 |

## Kubernetes 支持

仓库支持将当前无状态服务部署到 Kubernetes，并可选在集群内部署报告数据库。

| 组件 | 类型 | K8s 支持 | 说明 |
|------|------|-----------|------|
| API Gateway | 无状态 | 是 | 通过 Helm 部署（`charts/cicd/`） |
| Validation Service | 无状态 | 是 | 同 API Gateway |
| Orchestrator Service | 无状态 | 是 | 同 API Gateway |
| Worker Service | 无状态 | 是 | 需要节点可访问 Docker Socket |
| Reporting Service | 无状态 | 是 | 同 API Gateway |
| MySQL 8 报告库 | 有状态 | 可选 | 可集群内 StatefulSet + PVC，或外置 |

### Kubernetes 部署模式

- **全量集群内**：`mysql.enabled=true`，无状态服务 + MySQL + migration Job 全部在集群内
- **混合模式**：`mysql.enabled=false`，通过 `externalDatabase.*` / `externalDatabase.url` 指向外部数据库

集群内服务间通信使用 Kubernetes Service 与以下环境变量：

- `VALIDATION_URL`
- `ORCHESTRATOR_URL`
- `REPORTING_URL`
- `WORKER_URL`
- `DATABASE_URL`

Helm 打包/安装/排障请参考：[`charts/cicd/README.md`](https://github.com/xueyulinn/cicd-system/blob/review/charts/cicd/README.md)

**单一配置来源**：本地 Compose 使用 `compose.values.env`，由 `charts/cicd/values.yaml` 生成（`ruby scripts/gen-compose-env-from-values.rb`）。

### 请求去重

Orchestrator Service 对在途（`queued` / `running`）的相同运行请求去重：

- 第二个相同请求到来时，不新建运行
- 复用最早的在途运行，并返回该 `run_no`

去重键由 `pipeline name + YAML 内容 + commit + repo URL` 组成；不会包含调用方临时工作目录路径。

### Kubernetes 中的仓库运行

在 repo-backed 模式下，Worker 会在集群内先拉取指定仓库版本，再执行任务容器。公开仓库无需额外配置；私有仓库需注入凭据。

Helm 可配置以下方式之一：

- `workerService.gitAuth.githubToken`
- `workerService.gitAuth.username` + `workerService.gitAuth.password`
- `workerService.gitAuth.existingSecret`

## 可观测性

所有服务覆盖三大可观测性支柱：指标、日志、追踪，并通过 Docker Compose 一起部署。

### 组件栈

| 角色 | 组件 | 配置路径 |
|------|------|----------|
| 指标采集与存储 | Prometheus | `observability/prometheus/prometheus.yml` |
| 日志聚合 | Loki（经 OpenTelemetry Collector） | `observability/loki/loki-config.yml`, `observability/otel-collector/config.yml` |
| 分布式追踪 | Tempo | `observability/tempo/tempo.yml` |
| 遥测路由 | OpenTelemetry Collector | `observability/otel-collector/config.yml` |
| 可视化 | Grafana | `observability/grafana/` |

当前实现中，服务通过 OTLP 将日志/指标/追踪发往 OTel Collector；Collector 将追踪转发到 Tempo、暴露可被 Prometheus 抓取的指标端点，并将日志转发到 Loki。Grafana 统一查询 Prometheus、Loki、Tempo。

### 指标

Prometheus 抓取 OTel Collector 指标端点（Compose 下为 `otel-collector:8889`）。

当前主要指标来源于 OpenTelemetry HTTP 自动埋点：

- 入站 HTTP 指标：`TracingMiddleware`（`otelhttp.NewHandler`）
- 出站 HTTP 指标：`NewInstrumentedHTTPClient`（`otelhttp.NewTransport`）

当前代码中尚未注册 `cicd_*` 这类业务自定义指标。

### 结构化日志

所有服务使用 Go `log/slog` 输出结构化日志，并通过 OpenTelemetry 桥接。按需附加 `pipeline`、`run_no`、`stage`、`job`、`trace_id`、`span_id` 等上下文字段。

### 分布式追踪

服务使用 OpenTelemetry SDK 和 W3C Trace Context。主要 span 包括：

- **`pipeline.run`**：流水线运行根 span（包含 `pipeline`、`run_no`）
- **`stage.run`**：阶段级 span
- **`job.run`**：Worker 中的任务级 span
- **`mq.job.publish`**：Orchestrator 发布任务消息
- **`mq.job.consume`**：Worker 消费任务消息

服务间 HTTP 调用自动透传 trace context。`trace-id` 会落库并体现在 `report --run` 输出中，便于在 Grafana/Tempo 关联排障。

### 访问可观测性系统

执行 `docker compose --env-file compose.values.env up -d` 后：

| 工具 | 地址 | 账号 |
|------|------|------|
| Grafana | http://localhost:3000 | admin / admin |
| Prometheus | http://localhost:9090 | — |
| Tempo（API） | http://localhost:3200 | — |
| Loki（API） | http://localhost:3100 | — |

Grafana 已预置仪表盘：

- `Pipeline Overview`
- `Stage and Job Breakdown`
- `Logs Viewer`
- `Trace Explorer`
- `Parallel execution & RabbitMQ`

### 配置

可观测性由 `docker-compose.yaml` 中以下环境变量控制：

| 变量 | 作用 |
|------|------|
| `SERVICE_NAME` | 标识日志与追踪中的服务名 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTel Collector 端点（日志/指标/追踪） |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | 传输协议（`http/protobuf`） |

全部可观测性配置文件位于 `observability/`，共享埋点库位于 `internal/observability/`。

## 开发

### 本地全栈运行（Docker Compose）

推荐方式：一次启动全部服务、数据库、迁移与可观测性组件。

```bash
# 从 charts/cicd/values.yaml 生成 compose.values.env
ruby scripts/gen-compose-env-from-values.rb

# 启动所有容器（docker-compose.override.yaml 会从本地源码构建服务镜像）
docker compose --env-file compose.values.env up -d

# 查看运行状态
docker compose --env-file compose.values.env ps

# 查看日志
docker compose --env-file compose.values.env logs -f execution-service worker-service
```

`docker-compose.yaml` 读取 `compose.values.env`；`docker-compose.override.yaml` 自动追加 `build:`，用于本地源码构建。

- `docker compose --env-file compose.values.env up -d`：本地源码构建运行（开发）
- `docker compose --env-file compose.values.env up -d --build`：代码更新后强制重建
- `docker compose --env-file compose.values.env -f docker-compose.yaml up -d`：仅使用镜像（CI/生产）

可选 Worker 任务容器资源限制：

- `WORKER_JOB_CPU_LIMIT`（如 `0.5`、`1`、`2`）
- `WORKER_JOB_NANO_CPUS`（精确 `NanoCPUs`，不可与上项同时配置）
- `WORKER_JOB_MEMORY_LIMIT_MB`（内存 MB）

### 报告数据库

`report` 子命令依赖 MySQL 8（执行服务与报告服务都需要）。Compose 默认会自动处理 MySQL 与 migration。

手动准备示例：

```bash
docker compose --env-file compose.values.env up -d mysql db-migrate
export DATABASE_URL="cicd:cicd@tcp(localhost:3306)/reportstore?parseTime=true&charset=utf8mb4&loc=UTC"
```

详见 [dev-docs/report-db-setup.md](dev-docs/report-db-setup.md)。

### 项目结构

```text
e-team/
├── cmd/                          # 应用入口
│   ├── cicd/                     # CLI（verify、dryrun、run、report）
│   ├── api-gateway/              # API Gateway（8000）
│   ├── validation-service/       # Validation Service（8001）
│   ├── execution-service/        # Orchestrator Service（8002）
│   ├── worker-service/           # Worker Service（8003）
│   └── reporting-service/        # Reporting Service（8004）
├── internal/
│   ├── cli/                      # CLI 命令与网关客户端
│   ├── models/                   # 数据模型与类型
│   ├── observability/            # 共享可观测性埋点
│   ├── store/                    # 数据访问层（报告存储）
│   └── services/                 # 网关、校验、编排、执行、报告服务
├── migrations/                   # SQL 迁移
├── observability/                # 可观测性配置
├── charts/cicd/                  # Kubernetes Helm Chart
├── k8s/                          # Kubernetes 说明
├── scripts/                      # 开发脚本
├── .pipelines/                   # 流水线配置
├── compose.values.env            # Compose 环境变量（由 chart values 生成）
├── docker-compose.yaml           # 全栈编排
├── docker-compose.override.yaml  # 本地开发覆盖（从源码构建）
└── Makefile                      # 构建自动化
```

### 运行测试

```bash
# 运行全部测试
go test -v ./internal/...

# 覆盖率
go test -coverprofile=coverage.out ./internal/...
go tool cover -html=coverage.out -o coverage.html

# 打开覆盖率报告
start coverage.html
```

## 校验规则

工具当前执行以下规则：

1. **Git 仓库检查**：必须在 Git 仓库内执行
2. **阶段定义检查**：所有阶段必须先定义后使用
3. **任务引用检查**：`needs` 中引用的任务必须存在且位于同阶段
4. **环依赖检查**：不允许循环依赖
5. **阶段归属检查**：每个任务必须属于有效阶段
6. **YAML 语法检查**：文件必须是合法 YAML

## 错误示例

### 非法配置

```yaml
# 缺少阶段定义
stages: []
job-name:
  - stage: undefined_stage
```

**错误输出：**

```text
.pipelines/pipeline.yaml:8:3: job 'job-name' references undefined stage 'undefined_stage'
```

### 环依赖

```yaml
stages: [build]
job-a:
  - stage: build
  - needs: [job-b]
job-b:
  - stage: build
  - needs: [job-a]
```

**错误输出：**

```text
.pipelines/pipeline.yaml: circular dependency detected between jobs: job-a -> job-b -> job-a
```

## 技术栈

- **语言**：Go 1.25.6
- **CLI 框架**：Cobra
- **YAML 解析**：`gopkg.in/yaml.v3`
- **数据库**：MySQL 8 + `go-sql-driver/mysql`
- **可观测性**：OpenTelemetry SDK、Prometheus、Loki、Tempo、Grafana
- **容器**：Docker、Docker Compose
- **Kubernetes**：Helm、Kustomize、原生清单
- **测试**：Go 标准测试框架

## 参与贡献

1. Fork 本仓库
2. 创建功能分支
3. 提交改动
4. 为新功能补充测试
5. 运行 `go test -coverprofile=coverage.out ./internal/...`
6. 提交 Pull Request

## 文档索引

- [Feature status](FeatureStatus.md)
- [System design](dev-docs/design/high-level-design.md)
- [CLI reference](dev-docs/cli-reference.md)
- [Report database schema](dev-docs/design/design-db-schema.md)
- [Control component API reference](dev-docs/control-component-api.md)
- [Evaluator verification steps](dev-docs/verification-steps.md)

## License

本项目使用 [LICENSE](LICENSE) 中定义的许可协议。

## Team

本项目由 e-team 为 CS7580 SEA-SP26 开发，并由 yulinxue 持续维护改进。
