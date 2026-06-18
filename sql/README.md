# workspace-service MySQL schema

All business tables use the **`ws_`** prefix (workspace-service).

| File | Table | Status |
|------|-------|--------|
| `001_create_ws_file_node.sql` | `ws_file_node` | **in use** — inode operator + node_type |
| `002_create_ws_kernel_session.sql` | `ws_kernel_session` | **in use** — Jupyter session registry |
| `003_create_ws_recycle_item.sql` | `ws_recycle_item` | reserved P1 |
| `004_create_ws_git_operation.sql` | `ws_git_operation` | reserved P2 |
| `005_create_ws_favorite.sql` | `ws_favorite` | reserved P1 |
| `006_create_ws_share_acl.sql` | `ws_share_acl` | reserved P1 |
| `007_create_ws_file_intent.sql` | `ws_file_intent` | **in use** — write-ahead journal for file create/write |
| `099_migrate_legacy_table_names.sql` | — | rename `file_node` / `kernel_session` |

Apply all:

```bash
./scripts/apply-schema.sh workspace
```
