基于 Go 实现的联邦学习模型更新谱系一致性校验后端服务项目，一款后端服务，完成全局模型轮次登记、客户端更新父模型谱系校验、分叉与重放检测隔离以及不可变聚合谱系快照发布。

# BENZHI 评测说明

本服务校验多轮联邦聚合中客户端更新的合法祖先关系（模型参数摘要、轮次父节点、
聚合输入一致性），不是训练调度器，而是一个可验证谱系的后端与封存系统。

## 启动（长驻 HTTP 服务）

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/fedlineage --addr :8080 --db fedlineage.db
```

## 一键自检（评测契约）

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/fedlineage --smoke-test
```

退出码 0 表示通过：真实创建轮次/模型节点/更新，调用核心逻辑，关闭并重新打开
数据库验证持久化与重启恢复。

## Docker 双架构

```bash
bash build_benzhi_docker.sh my-fedlineage linux/amd64
docker run --rm --platform linux/amd64 my-fedlineage --smoke-test
bash build_benzhi_docker.sh my-fedlineage linux/arm64
docker run --rm --platform linux/arm64 my-fedlineage --smoke-test
```

构建脚本只负责构建镜像；镜像默认入口是 `/app/fedlineage`，默认参数为
`--smoke-test`，也可以显式传入该参数完成重启恢复自检。

镜像 ENTRYPOINT 固定为 `/app/fedlineage`，默认 CMD 为 `--smoke-test`；
评测仅传 flag，不传 `/app/fedlineage` 路径参数。

## HTTP API（前缀 /api）

- 模型节点：`POST /api/models/register`、`POST /api/models/confirm`、
  `POST /api/models/stale`、`POST /api/models/conflict`、
  `POST /api/models/parents`、`POST /api/models/children`、`GET /api/models`
- 轮次：`POST /api/rounds/register`、`POST /api/rounds/open`、
  `POST /api/rounds/close`、`POST /api/rounds/aggregable`、
  `POST /api/rounds/seal`、`POST /api/rounds/stats`、`GET /api/rounds`
- 更新：`POST /api/updates/receive`、`GET /api/updates`、
  `POST /api/updates/isolate`
- 谱系校验：`POST /api/lineage/verify`、`POST /api/lineage/verify-round`、
  `GET /api/lineage/forks`、`POST /api/lineage/ancestors`
- 聚合：`POST /api/aggregate/compute`、`POST /api/aggregate/confirm`、
  `POST /api/aggregate/audit`
- 快照：`POST /api/snapshots/publish`、`GET /api/snapshots`、
  `GET /api/snapshots/published`、`POST /api/snapshots/supersede`
- 自检：`GET /api/selfcheck`、`GET /api/health`
