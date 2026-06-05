#!/usr/bin/env bash
# Local smoke test: workspace-service :8080 + Kernel Gateway :8888
# Prereqs: Colima/Docker jkg on 8888, workspace-service running, mount_root + MySQL configured.
set -euo pipefail

BASE_WS="${BASE_WS:-http://127.0.0.1:8080}"
BASE_KG="${BASE_KG:-http://127.0.0.1:8888}"

echo "== health =="
curl -sf "$BASE_WS/healthz" >/dev/null && echo "healthz OK"
curl -sf "$BASE_WS/readyz" >/dev/null && echo "readyz OK"

echo "== gateway proxy =="
curl -sf "$BASE_WS/api/kernelspecs" | head -c 80 && echo " ... kernelspecs OK"
KERNEL_JSON=$(curl -sf -X POST "$BASE_WS/api/kernels" \
  -H "Content-Type: application/json" -d '{"name":"python3"}')
KERNEL_ID=$(echo "$KERNEL_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "kernel id=$KERNEL_ID"
curl -sf "$BASE_WS/api/kernels/$KERNEL_ID" >/dev/null && echo "get kernel OK"
curl -sf -o /dev/null -w "interrupt:%{http_code}\n" -X POST "$BASE_WS/api/kernels/$KERNEL_ID/interrupt"
curl -sf "$BASE_WS/api/kernels/$KERNEL_ID/restart" >/dev/null && echo "restart OK"
curl -sf -o /dev/null -w "delete:%{http_code}\n" -X DELETE "$BASE_WS/api/kernels/$KERNEL_ID"

CTX='{"owner_uin":"100001","uin":"200001","app_id":"260073493","workspace_id":"ws-test"'

echo "== file api =="
curl -sf -X POST "$BASE_WS/CreateFolder" -H "Content-Type: application/json" \
  -d "${CTX},\"path\":\"smoke\"}" | python3 -m json.tool | head -5
curl -sf -X POST "$BASE_WS/CreateFile" -H "Content-Type: application/json" \
  -d "${CTX},\"path\":\"smoke/t.txt\",\"content_base64\":\"dGVzdA==\"}" >/dev/null
echo "CreateFile OK"
curl -sf -X POST "$BASE_WS/CreateNotebook" -H "Content-Type: application/json" \
  -d "${CTX},\"path\":\"smoke/demo\"}" | python3 -m json.tool | head -8
echo "CreateNotebook OK"
curl -sf -X POST "$BASE_WS/ValidatePath" -H "Content-Type: application/json" \
  -d "${CTX},\"parent_path\":\"smoke\",\"name\":\"t.txt\"}" | python3 -m json.tool | head -3
curl -sf -X POST "$BASE_WS/GetFolderNodePath" -H "Content-Type: application/json" \
  -d "${CTX},\"path\":\"smoke\"}" | python3 -m json.tool | head -8
curl -sf -X POST "$BASE_WS/DeletePath" -H "Content-Type: application/json" \
  -d "${CTX},\"path\":\"smoke/t.txt\",\"soft_delete\":true}" >/dev/null
curl -sf -X POST "$BASE_WS/ListRecycleBin" -H "Content-Type: application/json" \
  -d "${CTX}}" | python3 -m json.tool | head -8

echo "== git api =="
curl -sf -X POST "$BASE_WS/GetStatus" -H "Content-Type: application/json" \
  -d "${CTX},\"path\":\"gitignore\"}" | python3 -m json.tool | head -8
curl -sf -o /dev/null -w "CreateGitFolder:%{http_code}\n" -X POST "$BASE_WS/CreateGitFolder" \
  -H "Content-Type: application/json" \
  -d "${CTX},\"target_path\":\"smoke-git\",\"repo_url\":\"https://github.com/lightnine/mini.git\",\"branch\":\"main\"}" || true

echo "== session probe (informational) =="
curl -s -o /dev/null -w "8080 POST /api/sessions -> %{http_code}\n" -X POST "$BASE_WS/api/sessions" \
  -H "Content-Type: application/json" \
  -d '{"path":"/test.ipynb","type":"notebook","kernel":{"name":"python3"}}' || true
curl -s -o /dev/null -w "8888 POST /api/sessions -> %{http_code}\n" -X POST "$BASE_KG/api/sessions" \
  -H "Content-Type: application/json" \
  -d '{"path":"/test.ipynb","type":"notebook","kernel":{"name":"python3"}}' || true

echo "All smoke checks finished."
