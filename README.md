# workspace-service

WeData Studio workspace file/Git API and Jupyter gateway proxy.

## Quick start

```bash
cp conf/workspace-service.yaml.example conf/workspace-service.yaml
# edit mysql.dsn

mysql -u root -p workspace < sql/001_create_file_node.sql
mysql -u root -p workspace < sql/002_create_kernel_session.sql

go run ./cmd/server -config conf/workspace-service.yaml
```

See [docs/local-integration.md](docs/local-integration.md) for Kernel Gateway setup.
