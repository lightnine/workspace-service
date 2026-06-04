# 本地联调：workspace-service + Kernel Gateway

日常回归跑自动化脚本即可；本文档只做**一次性环境准备**说明。

## 前置

MySQL 需有 `file_node` 表：

```bash
mysql -u root -p workspace < sql/001_create_file_node.sql
```

若库中是更早期的表（缺少 `app_id` / `workspace_id`），需自行 `ALTER TABLE` 补齐这两列及 `idx_file_node_app_workspace` 索引。

| 组件 | 说明 |
|------|------|
| Colima / Docker | Kernel Gateway 容器 |
| MySQL `3306` | `conf/workspace-service.yaml` 中 `mysql.dsn` |
| 挂载目录 | `workspace.mount_root`（如 `~/mnt/studio`），需可写 |
| Go 1.24+ | 编译 workspace-service |

## 1. Kernel Gateway（:8888）

`minimal-notebook` 镜像默认无 `jupyter_kernel_gateway`，需先装再启动：

```bash
docker rm -f jkg 2>/dev/null || true
docker run -d --name jkg -p 8888:8888 quay.io/jupyter/minimal-notebook:latest \
  bash -c "pip install -q jupyter_kernel_gateway && \
  start.sh jupyter kernelgateway \
    --KernelGatewayApp.ip=0.0.0.0 \
    --KernelGatewayApp.port=8888 \
    --KernelGatewayApp.api=kernel_gateway.jupyter_websocket"

curl -s http://127.0.0.1:8888/api/kernelspecs | head
curl -s -X POST http://127.0.0.1:8888/api/kernels \
  -H "Content-Type: application/json" -d '{"name":"python3"}'
```

## 2. workspace-service（:8080）

`conf/workspace-service.yaml`：

```yaml
gateway:
  url: "http://127.0.0.1:8888"
workspace:
  mount_root: "~/mnt/studio"
mysql:
  dsn: "..."  # 本机 MySQL
```

```bash
cd workspace-service
go run ./cmd/server -config conf/workspace-service.yaml
```

日志中**不应**出现 `gateway url is empty`。健康检查：`curl http://127.0.0.1:8080/healthz`。

## 3. 请求上下文（必填）

所有 Verb+Noun 接口 JSON 需带：

```json
{
  "owner_uin": "100001",
  "uin": "200001",
  "app_id": "260073493",
  "workspace_id": "ws-test"
}
```

也可由网关注入 HTTP 头：`X-Wedata-Owner-Uin`、`X-Wedata-Uin`、`X-Wedata-App-Id`、`X-Wedata-Workspace-Id`（body 优先）。

业务路径相对用户目录，例如 `demo/a.txt` 会解析为 `{app_id}/{workspace_id}/users/{uin}/demo/a.txt`。

### 目录树 / 回收站（新增）

| 接口 | 说明 |
|------|------|
| `ListFiles` | 增强字段：`inode_id`、`owner_uin`、`creator_uin`、`node_type`（`file`/`directory`/`git_folder`）、`is_git_folder`、`file_id`（Git 目录为 path 的 base64） |
| `ValidatePath` | `parent_path` + `name` → `{exists}`，对应现网 `ValidateFileName` |
| `GetFolderNodePath` | `path` → `{nodes:[]}` 面包屑 |
| `DeletePath` | `soft_delete:true` 移入 `{user}/trash/`；`permanent:true` 硬删 |
| `ListRecycleBin` | 列出回收站 |
| `RestorePath` | `trash_path` 恢复；可选 `target_parent` |
| `EmptyRecycleBin` | 清空回收站 |

### Git（新增）

| 接口 | 说明 |
|------|------|
| `CreateGitFolder` | 异步克隆；返回 `status`（1 等待 / 2 克隆中 / 4 就绪 / 5 失败） |
| `GetGitFolderStatus` | 轮询克隆状态 |
| `StageFiles` / `UnstageFiles` | `all` 或 `files[]` |
| `Commit` | 仅提交（需先 stage） |
| `PushRepo` | 仅推送 |
| `CommitAndPush` | 仍支持；内部先 stage all 再 commit，可选 push |

## 4. Git（可选）

生产建议配置 `workspace.git_meta_root` 将 `.git` 放在 JuiceFS 外；`CloneRepo` **必须**传 `branch`（单分支、不拉 tags）。

## 5. 自动化冒烟

```bash
./scripts/local-smoke-test.sh
```

## 已知现象

- `GET /api/kernels` 列表：KG 常返回 **403**（直连 8888 亦然），不影响 create/get/delete。
- Session：直连 KG 的 `POST /api/sessions` 可能 **500**；本地验收以 **Kernel API** 为主。
- 完整架构与 minikube/Enterprise Gateway 见 [wedata3_studio_workspace_server_juicefs_architecture.md](./wedata3_studio_workspace_server_juicefs_architecture.md)。
