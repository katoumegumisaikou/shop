.PHONY: deps-up deps-down deps-reset migrate-up migrate-down migrate-status server-run

# ============================================================
# Docker 中间件（PostgreSQL + Redis）
# ============================================================

deps-up:
	docker compose up -d
	@echo "中间件已启动: PostgreSQL(5432) Redis(6379)"

deps-down:
	docker compose down

deps-reset:
	docker compose down -v
	docker compose up -d
	@echo "中间件已重建（数据已清空），请执行 make migrate-up 初始化表结构"

# ============================================================
# 数据库迁移（Goose）
# ============================================================

migrate-up:
	cd server && goose -dir migrations postgres "host=localhost user=shop password=shop dbname=shop port=5432 sslmode=disable" up

migrate-down:
	cd server && goose -dir migrations postgres "host=localhost user=shop password=shop dbname=shop port=5432 sslmode=disable" down

migrate-status:
	cd server && goose -dir migrations postgres "host=localhost user=shop password=shop dbname=shop port=5432 sslmode=disable" status

# ============================================================
# 应用启动
# ============================================================

server-run:
	cd server && go run ./cmd/api/main.go

# ============================================================
# 前端
# ============================================================

client-h5:
	cd client && npm run dev:h5

admin-run:
	cd admin && npm run dev
