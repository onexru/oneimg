#!/bin/sh
# 演示版容器入口
#
# 演示模式（DEMO_MODE=1）在应用侧启用全站只读闸门：禁止上传与一切写操作。
# 这里只负责把「游客可浏览（tourist=1）」与「免人机验证（verify_method=none）」
# 写入空白数据库 —— 用后台守护进程轮询 data.db 的 settings 行，
# 仅当 verify_method 仍为空（管理员从未配置过）时注入，幂等且不覆盖人工改动。
#
# 注意：不再开启 start_register —— 注册是写操作，演示模式下会被闸门 403 拦截，
# 开了反而让登录页出现无法完成操作的「注册」入口。

set -e

DB_PATH="${SQLITE_PATH:-./data/data.db}"

apply_demo_defaults() {
    python3 - "$DB_PATH" << 'PYEOF'
import os
import sqlite3
import sys

db_path = sys.argv[1]
if not os.path.exists(db_path):
    sys.exit(0)

conn = sqlite3.connect(db_path, timeout=5)
try:
    row = conn.execute("SELECT verify_method FROM settings WHERE id = 1").fetchone()
    if row is None:
        sys.exit(0)
    if row[0] == "":
        conn.execute("UPDATE settings SET verify_method = 'none', tourist = 1 WHERE id = 1")
        conn.commit()
        print("[demo] 已写入演示预设：verify_method=none, tourist=1（注册保持关闭：演示模式禁止写操作）", flush=True)
finally:
    conn.close()
PYEOF
}

wait_and_apply() {
    # 等数据库出现，再等 settings 行被应用首启创建；最多等 120 秒
    i=0
    while [ $i -lt 60 ]; do
        if [ -f "$DB_PATH" ] && python3 - "$DB_PATH" << 'PYEOF' 2>/dev/null
import sqlite3, sys
c = sqlite3.connect(sys.argv[1], timeout=3)
try:
    row = c.execute("SELECT verify_method FROM settings WHERE id = 1").fetchone()
    sys.exit(0 if row is not None else 1)
except Exception:
    sys.exit(1)
PYEOF
        then
            apply_demo_defaults
            return 0
        fi
        sleep 2
        i=$((i + 1))
    done
    echo "[demo] 等待数据库超时，跳过演示预设注入" >&2
}

if ! command -v python3 >/dev/null 2>&1; then
    apk add --no-cache python3 >/dev/null 2>&1
    PYTHON_INSTALLED=1
fi

wait_and_apply &

[ -n "$PYTHON_INSTALLED" ] && (sleep 30 && apk del python3 >/dev/null 2>&1) &

exec "$@"
